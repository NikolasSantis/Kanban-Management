package integration_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bytes mínimos que o http.DetectContentType reconhece pela assinatura.
var (
	pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x01, 0x02, 0x03}, 50)...)
	pdfBytes = append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 100)...)
	zipBytes = append([]byte("PK\x03\x04"), bytes.Repeat([]byte{0x00}, 100)...)
	exeBytes = append([]byte("MZ\x90\x00"), bytes.Repeat([]byte{0x00}, 100)...)
)

type attachmentScenario struct {
	commentScenario
}

func newAttachmentScenario(t *testing.T) attachmentScenario {
	t.Helper()

	return attachmentScenario{newCommentScenario(t)}
}

func (s attachmentScenario) path() string { return "/attachment/card/" + s.cardID }

func (s attachmentScenario) upload(t *testing.T, token, name string, content []byte) (int, testutil.Envelope) {
	t.Helper()

	return testutil.Upload(t, s.app, s.path(), token, name, content)
}

func (s attachmentScenario) uploadOK(t *testing.T, token, name string, content []byte) map[string]any {
	t.Helper()

	status, env := s.upload(t, token, name, content)
	require.Equal(t, 201, status, "upload %s: %v", name, env.Error)

	return env.Map()
}

func TestUploadAttachment_Success(t *testing.T) {
	s := newAttachmentScenario(t)

	attachment := s.uploadOK(t, s.member.Token, "foto.png", pngBytes)

	assert.Equal(t, "file", attachment["kind"])
	assert.Equal(t, "foto.png", attachment["file_name"])
	assert.Equal(t, "foto.png", attachment["original_name"])
	assert.Equal(t, "image/png", attachment["content_type"])
	assert.Equal(t, float64(len(pngBytes)), attachment["size_bytes"])
	assert.Equal(t, s.member.ID, attachment["uploaded_by"])
	assert.Equal(t, s.cardID, attachment["card_id"])
	assert.Equal(t, s.projectID, attachment["project_id"])
	assert.Equal(t, "/attachment/"+attachment["id"].(string)+"/download", attachment["download_url"])
	assert.NotContains(t, attachment, "storage_key", "a chave do storage nunca vai para o cliente")
	assert.NotContains(t, attachment, "url")
}

func TestUploadAttachment_AllowedTypes(t *testing.T) {
	s := newAttachmentScenario(t)

	cases := []struct {
		name        string
		fileName    string
		content     []byte
		wantType    string
		wantContent string
	}{
		{"pdf", "doc.pdf", pdfBytes, "application/pdf", ""},
		{"zip (docx/xlsx também aparecem assim)", "planilha.xlsx", zipBytes, "application/zip", ""},
		{"texto", "notas.txt", []byte("olá, mundo"), "text/plain; charset=utf-8", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attachment := s.uploadOK(t, s.admin.Token, tc.fileName, tc.content)

			assert.Equal(t, tc.wantType, attachment["content_type"])
		})
	}
}

func TestUploadAttachment_Rejections(t *testing.T) {
	s := newAttachmentScenario(t)

	t.Run("executável renomeado para .png é recusado (o conteúdo manda)", func(t *testing.T) {
		status, _ := s.upload(t, s.admin.Token, "foto.png", exeBytes)

		assert.Equal(t, 400, status)
	})

	t.Run("HTML disfarçado de .txt é recusado", func(t *testing.T) {
		status, _ := s.upload(t, s.admin.Token, "notas.txt", []byte("<html><body><script>alert(1)</script></body></html>"))

		assert.Equal(t, 400, status)
	})

	t.Run("arquivo vazio", func(t *testing.T) {
		status, _ := s.upload(t, s.admin.Token, "vazio.png", nil)

		assert.Equal(t, 400, status)
	})

	t.Run("sem o campo file", func(t *testing.T) {
		status, _ := testutil.UploadField(t, s.app, s.path(), s.admin.Token, "outro", "foto.png", pngBytes)

		assert.Equal(t, 400, status)
	})

	t.Run("passou do limite de 10 MB devolve 413", func(t *testing.T) {
		big := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("a"), 10*1024*1024)...)

		status, _ := s.upload(t, s.admin.Token, "grande.pdf", big)

		assert.Equal(t, 413, status)
	})

	t.Run("o limite do corpo da requisição fica 1 MB acima do limite do anexo", func(t *testing.T) {
		// Acima disso o próprio servidor recusa (413) antes de chegar ao handler;
		// o app.Test não consegue representar essa resposta, então confere-se a config.
		assert.Equal(t, 11*1024*1024, s.app.Config().BodyLimit)
	})

	t.Run("nada foi salvo pelos pedidos recusados", func(t *testing.T) {
		status, env := s.do(t, "GET", s.path(), s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Empty(t, env.List())
	})
}

func TestUploadAttachment_SanitizesTheFileName(t *testing.T) {
	s := newAttachmentScenario(t)

	attachment := s.uploadOK(t, s.admin.Token, "../../../evil.png", pngBytes)

	assert.Equal(t, "evil.png", attachment["file_name"])
	assert.Equal(t, "evil.png", attachment["original_name"])

	attachment = s.uploadOK(t, s.admin.Token, "relatório final.png", pngBytes)

	assert.Equal(t, "relatriofinal.png", attachment["file_name"], "nome seguro para o storage")
	assert.Equal(t, "relatório final.png", attachment["original_name"], "nome original só para exibição")
}

func TestUploadAttachment_Permissions(t *testing.T) {
	s := newAttachmentScenario(t)

	t.Run("member anexa", func(t *testing.T) {
		status, _ := s.upload(t, s.member.Token, "a.png", pngBytes)
		assert.Equal(t, 201, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.upload(t, s.outsider.Token, "a.png", pngBytes)
		assert.Equal(t, 404, status)
	})

	t.Run("sem autenticação", func(t *testing.T) {
		status, _ := s.upload(t, "", "a.png", pngBytes)
		assert.Equal(t, 401, status)
	})

	t.Run("id de card inválido e inexistente", func(t *testing.T) {
		status, _ := testutil.Upload(t, s.app, "/attachment/card/xyz", s.admin.Token, "a.png", pngBytes)
		assert.Equal(t, 400, status)

		status, _ = testutil.Upload(t, s.app, "/attachment/card/000000000000000000000001", s.admin.Token, "a.png", pngBytes)
		assert.Equal(t, 404, status)
	})
}

func TestUploadAttachment_LimitOf20PerCard(t *testing.T) {
	s := newAttachmentScenario(t)

	for i := 0; i < 20; i++ {
		status, env := s.do(t, "POST", s.path()+"/link", s.admin.Token, map[string]any{
			"name": fmt.Sprintf("link %d", i), "url": "https://exemplo.com/" + fmt.Sprint(i),
		})
		require.Equal(t, 201, status, "link %d: %v", i, env.Error)
	}

	status, _ := s.upload(t, s.admin.Token, "extra.png", pngBytes)
	assert.Equal(t, 409, status, "o 21º anexo (arquivo)")

	status, _ = s.do(t, "POST", s.path()+"/link", s.admin.Token, map[string]any{"name": "extra", "url": "https://exemplo.com"})
	assert.Equal(t, 409, status, "o 21º anexo (link)")
}

func TestDownloadAttachment(t *testing.T) {
	s := newAttachmentScenario(t)
	attachment := s.uploadOK(t, s.admin.Token, "relatório final.pdf", pdfBytes)
	downloadURL := attachment["download_url"].(string)

	t.Run("devolve os mesmos bytes, com tipo e nome originais", func(t *testing.T) {
		status, header, body := testutil.DoRaw(t, s.app, "GET", downloadURL, s.member.Token)

		require.Equal(t, 200, status)
		assert.Equal(t, sha256.Sum256(pdfBytes), sha256.Sum256(body), "o hash tem que ser idêntico")
		assert.Equal(t, "application/pdf", header.Get("Content-Type"))
		assert.True(t, strings.HasPrefix(header.Get("Content-Disposition"), "attachment;"), header.Get("Content-Disposition"))
		assert.Contains(t, header.Get("Content-Disposition"), "relat", "o nome original aparece (codificado se tiver acento)")
		assert.Equal(t, "nosniff", header.Get("X-Content-Type-Options"))
	})

	t.Run("nome simples vai direto no Content-Disposition", func(t *testing.T) {
		plain := s.uploadOK(t, s.admin.Token, "foto.png", pngBytes)

		_, header, _ := testutil.DoRaw(t, s.app, "GET", plain["download_url"].(string), s.admin.Token)

		assert.Equal(t, `attachment; filename=foto.png`, header.Get("Content-Disposition"))
	})

	t.Run("não-membro recebe 404 e sem autenticação 401", func(t *testing.T) {
		status, _, _ := testutil.DoRaw(t, s.app, "GET", downloadURL, s.outsider.Token)
		assert.Equal(t, 404, status)

		status, _, _ = testutil.DoRaw(t, s.app, "GET", downloadURL, "")
		assert.Equal(t, 401, status)
	})

	t.Run("id inválido e inexistente", func(t *testing.T) {
		status, _, _ := testutil.DoRaw(t, s.app, "GET", "/attachment/xyz/download", s.admin.Token)
		assert.Equal(t, 400, status)

		status, _, _ = testutil.DoRaw(t, s.app, "GET", "/attachment/000000000000000000000001/download", s.admin.Token)
		assert.Equal(t, 404, status)
	})
}

func TestLinkAttachment(t *testing.T) {
	s := newAttachmentScenario(t)
	path := s.path() + "/link"

	t.Run("link http/https vira anexo kind=link sem download_url", func(t *testing.T) {
		status, env := s.do(t, "POST", path, s.member.Token, map[string]any{"name": "  Especificação  ", "url": "https://exemplo.com/spec"})

		require.Equal(t, 201, status)
		link := env.Map()
		assert.Equal(t, "link", link["kind"])
		assert.Equal(t, "Especificação", link["file_name"])
		assert.Equal(t, "https://exemplo.com/spec", link["url"])
		assert.NotContains(t, link, "download_url")
		assert.NotContains(t, link, "storage_key")

		status, _, _ = download(t, s, link["id"].(string), s.admin.Token)
		assert.Equal(t, 400, status, "link não tem arquivo para baixar")
	})

	cases := []struct {
		name       string
		token      string
		body       any
		wantStatus int
	}{
		{"javascript: é recusado", s.admin.Token, map[string]any{"name": "x", "url": "javascript:alert(1)"}, 400},
		{"data: é recusado", s.admin.Token, map[string]any{"name": "x", "url": "data:text/html;base64,AAAA"}, 400},
		{"file: é recusado", s.admin.Token, map[string]any{"name": "x", "url": "file:///etc/passwd"}, 400},
		{"sem esquema", s.admin.Token, map[string]any{"name": "x", "url": "exemplo.com"}, 400},
		{"sem url", s.admin.Token, map[string]any{"name": "x"}, 400},
		{"sem nome", s.admin.Token, map[string]any{"url": "https://exemplo.com"}, 400},
		{"nome em branco", s.admin.Token, map[string]any{"name": "  ", "url": "https://exemplo.com"}, 400},
		{"corpo inválido", s.admin.Token, "texto", 400},
		{"não-membro recebe 404", s.outsider.Token, map[string]any{"name": "x", "url": "https://exemplo.com"}, 404},
		{"sem autenticação", "", map[string]any{"name": "x", "url": "https://exemplo.com"}, 401},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "POST", path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func download(t *testing.T, s attachmentScenario, attachmentID, token string) (int, map[string][]string, []byte) {
	t.Helper()

	status, header, body := testutil.DoRaw(t, s.app, "GET", "/attachment/"+attachmentID+"/download", token)

	return status, header, body
}

func TestGetAttachments(t *testing.T) {
	s := newAttachmentScenario(t)
	s.uploadOK(t, s.admin.Token, "primeiro.png", pngBytes)
	s.do(t, "POST", s.path()+"/link", s.member.Token, map[string]any{"name": "segundo", "url": "https://exemplo.com"})

	t.Run("member lista (mais novo primeiro), arquivos e links", func(t *testing.T) {
		status, env := s.do(t, "GET", s.path(), s.member.Token, nil)

		require.Equal(t, 200, status)
		list := env.List()
		require.Len(t, list, 2)
		assert.Equal(t, "link", list[0]["kind"])
		assert.Equal(t, "file", list[1]["kind"])
		assert.NotContains(t, string(env.Data), "storage_key")
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.do(t, "GET", s.path(), s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("card sem anexos devolve lista vazia", func(t *testing.T) {
		other := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "vazio")

		status, env := s.do(t, "GET", "/attachment/card/"+other, s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.JSONEq(t, "[]", string(env.Data))
	})
}

func TestDeleteAttachment(t *testing.T) {
	s := newAttachmentScenario(t)

	t.Run("member não apaga anexo de outro", func(t *testing.T) {
		attachment := s.uploadOK(t, s.admin.Token, "a.png", pngBytes)

		status, _ := s.do(t, "DELETE", "/attachment/"+attachment["id"].(string), s.member.Token, nil)

		assert.Equal(t, 403, status)
	})

	t.Run("quem enviou apaga o próprio", func(t *testing.T) {
		attachment := s.uploadOK(t, s.member.Token, "b.png", pngBytes)

		status, _ := s.do(t, "DELETE", "/attachment/"+attachment["id"].(string), s.member.Token, nil)

		assert.Equal(t, 200, status)
	})

	t.Run("manager apaga de qualquer um (moderação)", func(t *testing.T) {
		attachment := s.uploadOK(t, s.member.Token, "c.png", pngBytes)

		status, _ := s.do(t, "DELETE", "/attachment/"+attachment["id"].(string), s.manager.Token, nil)

		assert.Equal(t, 200, status)
	})

	t.Run("não-membro recebe 404 e id inválido 400", func(t *testing.T) {
		attachment := s.uploadOK(t, s.admin.Token, "d.png", pngBytes)

		status, _ := s.do(t, "DELETE", "/attachment/"+attachment["id"].(string), s.outsider.Token, nil)
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "DELETE", "/attachment/xyz", s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})

	t.Run("soft delete: some da lista, não baixa mais e não apaga duas vezes", func(t *testing.T) {
		attachment := s.uploadOK(t, s.admin.Token, "e.png", pngBytes)
		id := attachment["id"].(string)

		status, _ := s.do(t, "DELETE", "/attachment/"+id, s.admin.Token, nil)
		require.Equal(t, 200, status)

		_, env := s.do(t, "GET", s.path(), s.admin.Token, nil)
		for _, item := range env.List() {
			assert.NotEqual(t, id, item["id"])
		}

		status, _, _ = download(t, s, id, s.admin.Token)
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "DELETE", "/attachment/"+id, s.admin.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("apagar libera uma vaga no limite de 20", func(t *testing.T) {
		fresh := newAttachmentScenario(t)

		var firstID string
		for i := 0; i < 20; i++ {
			status, env := fresh.do(t, "POST", fresh.path()+"/link", fresh.admin.Token, map[string]any{"name": fmt.Sprint(i), "url": "https://exemplo.com"})
			require.Equal(t, 201, status)
			if i == 0 {
				firstID = env.Map()["id"].(string)
			}
		}

		status, _ := fresh.do(t, "DELETE", "/attachment/"+firstID, fresh.admin.Token, nil)
		require.Equal(t, 200, status)

		status, _ = fresh.upload(t, fresh.admin.Token, "novo.png", pngBytes)
		assert.Equal(t, 201, status)
	})
}
