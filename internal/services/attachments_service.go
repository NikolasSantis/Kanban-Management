package services

import (
	"context"
	"errors"
	"mime"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"kanban-management/internal/database"
	"kanban-management/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	MaxAttachmentsPerCard   = 20
	MaxAttachmentNameLength = 100
	MaxOriginalNameLength   = 255
	MaxLinkURLLength        = 2000
)

// ErrInvalidLink: nome fora de 1–100 caracteres ou URL que não é http/https.
var ErrInvalidLink = errors.New("invalid link")

var allowedContentTypes = map[string]bool{
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/webp":      true,
	"application/pdf": true,
	"text/plain":      true,
	"application/zip": true,
}

var invalidFileNameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeFileName devolve um nome seguro para usar na chave do storage:
// só o último segmento do caminho (barras e contrabarras), apenas
// [a-zA-Z0-9._-], sem pontos no começo, até 100 caracteres mantendo a extensão.
// Nunca devolve vazio ("file" é o nome de reserva).
func SanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = invalidFileNameChars.ReplaceAllString(name, "")
	name = strings.TrimLeft(name, ".")

	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)

	if len(ext) > 20 {
		ext = ext[:20]
	}

	if base == "" {
		base = "file"
	}

	if limit := MaxAttachmentNameLength - len(ext); len(base) > limit {
		base = base[:limit]
	}

	return base + ext
}

// DisplayName limpa o nome original só para exibição: último segmento do
// caminho, sem caracteres de controle, até 255 caracteres.
func DisplayName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)

	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}

		return r
	}, name)

	name = strings.TrimSpace(name)

	if name == "" || name == "." || name == "/" {
		return "file"
	}

	if utf8.RuneCountInString(name) > MaxOriginalNameLength {
		name = string([]rune(name)[:MaxOriginalNameLength])
	}

	return name
}

// IsAllowedContentType confere o tipo (ex.: "text/plain; charset=utf-8") na allowlist.
func IsAllowedContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)

	if err != nil {
		return false
	}

	return allowedContentTypes[mediaType]
}

// BuildStorageKey monta cards/<card_id>/<attachment_id>/<nome-sanitizado>.
// O nome enviado pelo cliente nunca é usado como caminho.
func BuildStorageKey(cardID, attachmentID bson.ObjectID, fileName string) string {
	return "cards/" + cardID.Hex() + "/" + attachmentID.Hex() + "/" + SanitizeFileName(fileName)
}

// ValidateLink valida o nome (1–100) e a URL (http/https com host, até 2000 caracteres).
func ValidateLink(name, rawURL string) (string, string, error) {
	name = strings.TrimSpace(name)

	if length := utf8.RuneCountInString(name); length < 1 || length > MaxAttachmentNameLength {
		return "", "", ErrInvalidLink
	}

	rawURL = strings.TrimSpace(rawURL)

	if rawURL == "" || len(rawURL) > MaxLinkURLLength {
		return "", "", ErrInvalidLink
	}

	parsed, err := url.Parse(rawURL)

	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", ErrInvalidLink
	}

	return name, rawURL, nil
}

// FindActiveAttachment busca um anexo que não foi deletado.
func FindActiveAttachment(ctx context.Context, attachmentID bson.ObjectID) (*models.Attachment, error) {
	var attachment models.Attachment

	err := database.GetCollection("attachments").FindOne(ctx, bson.M{
		"_id": attachmentID,
		"$or": NotDeleted(),
	}).Decode(&attachment)

	if err != nil {
		return nil, err
	}

	return &attachment, nil
}

// ActiveAttachmentCount conta os anexos ativos do card.
func ActiveAttachmentCount(ctx context.Context, cardID bson.ObjectID) (int64, error) {
	return database.GetCollection("attachments").CountDocuments(ctx, bson.M{
		"card_id": cardID,
		"$or":     NotDeleted(),
	})
}
