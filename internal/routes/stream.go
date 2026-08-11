package routes

import (
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/stream"
	"EverythingSuckz/fsb/internal/types"
	"EverythingSuckz/fsb/internal/utils"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gotd/td/tg"
	range_parser "github.com/quantumsheep/range-parser"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

var log *zap.Logger

func (e *allRoutes) LoadHome(r *Route) {
	log = e.log.Named("Stream")
	defer log.Info("Loaded stream route")
	r.Engine.GET("/stream/:messageID", getStreamRoute)
	r.Engine.HEAD("/stream/:messageID", getStreamRoute)
}

func getStreamRoute(ctx *gin.Context) {
	w := ctx.Writer
	r := ctx.Request

	messageIDParm := ctx.Param("messageID")
	messageID, err := strconv.Atoi(messageIDParm)
	if err != nil || messageID <= 0 {
		http.Error(w, "invalid message ID", http.StatusBadRequest)
		return
	}

	authHash := ctx.Query("hash")
	if authHash == "" {
		http.Error(w, "missing hash param", http.StatusBadRequest)
		return
	}

	worker := bot.GetNextWorker()

	file, err := utils.TimeFuncWithResult(log, "FileFromMessage", func() (*types.File, error) {
		return utils.FileFromMessage(ctx, worker.Client, messageID)
	})
	if err != nil {
		log.Warn("Failed to resolve stream message", zap.Int("messageID", messageID), zap.Error(err))
		http.Error(w, "file not found or link invalid", http.StatusNotFound)
		return
	}

	expectedHash := utils.PackFile(
		file.FileName,
		file.FileSize,
		file.MimeType,
		file.ID,
	)
	if !utils.CheckHash(authHash, expectedHash) {
		http.Error(w, "file not found or link invalid", http.StatusNotFound)
		return
	}

	// for photo messages
	if file.FileSize == 0 {
		res, err := worker.Client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: file.Location,
			Offset:   0,
			Limit:    1024 * 1024,
		})
		if err != nil {
			log.Error("Failed to download photo", zap.Int("messageID", messageID), zap.Error(err))
			http.Error(w, "failed to stream file", http.StatusBadGateway)
			return
		}
		result, ok := res.(*tg.UploadFile)
		if !ok {
			http.Error(w, "unexpected response", http.StatusInternalServerError)
			return
		}
		fileBytes := result.GetBytes()
		start, end, status, err := parseRequestedRange(int64(len(fileBytes)), r.Header.Get("Range"))
		if err != nil {
			ctx.Header("Content-Range", fmt.Sprintf("bytes */%d", len(fileBytes)))
			http.Error(w, "requested range is not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		setFileHeaders(ctx, file.FileName, file.MimeType, end-start+1, "inline")
		if status == http.StatusPartialContent {
			ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(fileBytes)))
		}
		if r.Method == http.MethodHead {
			ctx.Status(status)
			return
		}
		ctx.Data(status, normalizedMIME(file.MimeType), fileBytes[start:end+1])
		return
	}

	start, end, status, err := parseRequestedRange(file.FileSize, r.Header.Get("Range"))
	if err != nil {
		ctx.Header("Content-Range", fmt.Sprintf("bytes */%d", file.FileSize))
		http.Error(w, "requested range is not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if status == http.StatusPartialContent {
		ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.FileSize))
		log.Info("Content-Range", zap.Int64("start", start), zap.Int64("end", end), zap.Int64("fileSize", file.FileSize))
	}

	contentLength := end - start + 1
	disposition := "inline"
	if ctx.Query("d") == "true" {
		disposition = "attachment"
	}
	setFileHeaders(ctx, file.FileName, file.MimeType, contentLength, disposition)

	if r.Method == http.MethodHead {
		ctx.Status(status)
		return
	}
	ctx.Status(status)
	pipe, err := stream.NewStreamPipe(ctx, worker.Client, file.Location, start, end, log)
	if err != nil {
		log.Error("Failed to create stream pipe", zap.Error(err))
		return
	}
	defer pipe.Close()
	if _, err := io.CopyN(w, pipe, contentLength); err != nil {
		if !utils.IsClientDisconnectError(err) {
			log.Error("Error while copying stream", zap.Error(err))
		}
	}
}

func parseRequestedRange(size int64, rangeHeader string) (start, end int64, status int, err error) {
	if size <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid file size: %d", size)
	}
	if rangeHeader == "" {
		return 0, size - 1, http.StatusOK, nil
	}
	ranges, err := range_parser.Parse(size, rangeHeader)
	if err != nil || len(ranges) != 1 {
		return 0, 0, 0, fmt.Errorf("unsupported or invalid range")
	}
	return ranges[0].Start, ranges[0].End, http.StatusPartialContent, nil
}

func normalizedMIME(mimeType string) string {
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

func setFileHeaders(ctx *gin.Context, fileName, mimeType string, contentLength int64, disposition string) {
	fileName = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, path.Base(strings.ReplaceAll(fileName, "\\", "/")))
	if fileName == "" || fileName == "." {
		fileName = "file"
	}
	ctx.Header("Accept-Ranges", "bytes")
	ctx.Header("Content-Type", normalizedMIME(mimeType))
	ctx.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	ctx.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": fileName}))
}
