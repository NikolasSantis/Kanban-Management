package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	AttachmentKindFile = "file"
	AttachmentKindLink = "link"
)

type Attachment struct {
	ID         *bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID  *bson.ObjectID `bson:"project_id" json:"project_id"`
	BoardID    *bson.ObjectID `bson:"board_id" json:"board_id"`
	CardID     *bson.ObjectID `bson:"card_id" json:"card_id"`
	UploadedBy *bson.ObjectID `bson:"uploaded_by" json:"uploaded_by"`
	Kind       string         `bson:"kind" json:"kind"` // "file" | "link"
	// FileName é o nome sanitizado (o que vai na chave do storage). Em links, é o nome informado.
	FileName string `bson:"file_name" json:"file_name"`
	// OriginalName é o nome enviado pelo cliente (só para exibição e download).
	OriginalName string `bson:"original_name" json:"original_name"`
	// ContentType é o tipo DETECTADO pelo conteúdo.
	ContentType string `bson:"content_type,omitempty" json:"content_type,omitempty"`
	// ClientContentType é o tipo informado pelo cliente (dado informativo).
	ClientContentType string `bson:"client_content_type,omitempty" json:"client_content_type,omitempty"`
	SizeBytes         int64  `bson:"size_bytes,omitempty" json:"size_bytes,omitempty"`
	// StorageKey nunca vai para o cliente.
	StorageKey string     `bson:"storage_key,omitempty" json:"-"`
	URL        string     `bson:"url,omitempty" json:"url,omitempty"` // só para kind=link
	CreatedAt  time.Time  `bson:"created_at" json:"created_at"`
	DeletedAt  *time.Time `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

// AttachmentResponse acrescenta o download_url (só para kind=file).
type AttachmentResponse struct {
	Attachment
	DownloadURL string `json:"download_url,omitempty"`
}

func ToAttachmentResponse(attachment Attachment) AttachmentResponse {
	response := AttachmentResponse{Attachment: attachment}

	if attachment.Kind == AttachmentKindFile && attachment.ID != nil {
		response.DownloadURL = "/attachment/" + attachment.ID.Hex() + "/download"
	}

	return response
}

func ToAttachmentResponses(attachments []Attachment) []AttachmentResponse {
	responses := make([]AttachmentResponse, 0, len(attachments))

	for _, attachment := range attachments {
		responses = append(responses, ToAttachmentResponse(attachment))
	}

	return responses
}
