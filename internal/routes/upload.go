package routes

import (
    "bytes"
    "crypto/subtle"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/textproto"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/rguedesdev/TG-FileStreamBot/config"
    "github.com/rguedesdev/TG-FileStreamBot/internal/bot"
    "github.com/rguedesdev/TG-FileStreamBot/internal/utils"
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

// The hosted Telegram Bot API currently accepts uploads up to 50 MB.
const maxUploadBytes = 50_000_000

func (e *allRoutes) LoadUpload(r *Route) {
    r.Engine.POST("/api/upload", uploadFileRoute)
}

func uploadFileRoute(ctx *gin.Context) {
    token := config.ValueOf.UploadToken
    supplied := ctx.GetHeader("X-Upload-Token")
    if token == "" || len(token) != len(supplied) || subtle.ConstantTimeCompare([]byte(token), []byte(supplied)) != 1 {
        ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxUploadBytes+1024*1024)
    file, header, err := ctx.Request.FormFile("file")
    if err != nil {
        ctx.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
        return
    }
    defer file.Close()
    if header.Size <= 0 || header.Size > maxUploadBytes {
        ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file must be between 1 byte and 50 MB"})
        return
    }
    content, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
    if err != nil || len(content) > maxUploadBytes {
        ctx.JSON(http.StatusBadRequest, gin.H{"error": "could not read uploaded file"})
        return
    }

    var body bytes.Buffer
    writer := multipart.NewWriter(&body)
    if err = writer.WriteField("chat_id", fmt.Sprintf("-100%d", config.ValueOf.LogChannelID)); err == nil {
        var part io.Writer
        mimeHeader := make(textproto.MIMEHeader)
        mimeHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="document"; filename=%q`, header.Filename))
        mimeHeader.Set("Content-Type", header.Header.Get("Content-Type"))
        part, err = writer.CreatePart(mimeHeader)
        if err == nil {
            _, err = part.Write(content)
        }
    }
    if err == nil {
        err = writer.Close()
    }
    if err != nil {
        ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not prepare Telegram upload"})
        return
    }

    endpoint := "https://api.telegram.org/bot" + config.ValueOf.BotToken + "/sendDocument"
    client := &http.Client{Timeout: 2 * time.Minute}
    var result struct {
        OK          bool   `json:"ok"`
        Description string `json:"description"`
        ErrorCode   int    `json:"error_code"`
        Parameters  struct {
            RetryAfter int `json:"retry_after"`
        } `json:"parameters"`
        Result      struct { MessageID int `json:"message_id"` } `json:"result"`
    }
    var responseStatus int
    var telegramResponseBody string
    for attempt := 0; ; attempt++ {
        request, requestErr := http.NewRequestWithContext(ctx.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body.Bytes()))
        if requestErr != nil {
            ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not create Telegram request: %v", requestErr)})
            return
        }
        request.Header.Set("Content-Type", writer.FormDataContentType())
        response, requestErr := client.Do(request)
        if requestErr != nil {
            utils.Logger.Error("Telegram upload request failed", zap.Error(requestErr))
            ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Telegram upload request failed: %v", requestErr)})
            return
        }
        responseStatus = response.StatusCode
        responseBytes, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024))
        response.Body.Close()
        telegramResponseBody = strings.TrimSpace(string(responseBytes))
        if readErr != nil {
            utils.Logger.Error("Could not read Telegram upload response", zap.Int("statusCode", responseStatus), zap.Error(readErr))
            ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Could not read Telegram upload response: %v", readErr)})
            return
        }
        decodeErr := json.Unmarshal(responseBytes, &result)
        if decodeErr != nil {
            utils.Logger.Error("Could not decode Telegram upload response", zap.Int("statusCode", responseStatus), zap.Error(decodeErr))
            ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Could not decode Telegram upload response: %v", decodeErr)})
            return
        }
        if result.OK && result.Result.MessageID > 0 {
            break
        }
        if (responseStatus == http.StatusTooManyRequests || result.ErrorCode == http.StatusTooManyRequests) && result.Parameters.RetryAfter > 0 && attempt < 5 {
            delay := time.Duration(result.Parameters.RetryAfter+1) * time.Second
            utils.Logger.Warn("Telegram rate limit reached; retrying upload", zap.Duration("wait", delay), zap.Int("attempt", attempt+1))
            timer := time.NewTimer(delay)
            select {
            case <-ctx.Request.Context().Done():
                timer.Stop()
                ctx.JSON(http.StatusRequestTimeout, gin.H{"error": "Upload canceled while waiting for Telegram rate limit."})
                return
            case <-timer.C:
            }
            continue
        }
        detail := result.Description
        if detail == "" {
            detail = fmt.Sprintf("Telegram recusou o arquivo sem descrição (HTTP %d, código %d)", responseStatus, result.ErrorCode)
            if telegramResponseBody != "" {
                detail += ": " + telegramResponseBody
            }
        }
        utils.Logger.Error("Telegram did not accept uploaded file", zap.Int("statusCode", responseStatus), zap.Int("errorCode", result.ErrorCode), zap.String("description", result.Description), zap.String("responseBody", telegramResponseBody))
        ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Telegram did not accept the file: %s", detail)})
        return
    }

    worker := bot.GetNextWorker()
    telegramFile, err := utils.FileFromMessage(ctx, worker.Client, result.Result.MessageID)
    if err != nil {
        utils.Logger.Error("Unable to resolve uploaded Telegram message", zap.Int("messageID", result.Result.MessageID), zap.Error(err))
        ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Telegram accepted the upload, but the file could not be resolved from the storage channel: %v", err)})
        return
    }
    fullHash := utils.PackFile(telegramFile.FileName, telegramFile.FileSize, telegramFile.MimeType, telegramFile.ID)
    hash := utils.GetShortHash(fullHash)
    url := fmt.Sprintf("%s/stream/%s?hash=%s", strings.TrimRight(config.ValueOf.Host, "/"), strconv.Itoa(result.Result.MessageID), hash)
    ctx.JSON(http.StatusCreated, gin.H{"url": url})
}
