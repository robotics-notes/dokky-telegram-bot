package main

import (
	"fmt"
	"github.com/docsbox/go-libreofficekit"
	"github.com/rakyll/magicmime"
	"gopkg.in/telegram-bot-api.v4"
	"image"
	"log"
	"os"
	"unsafe"
)

var (
	TelegramBotToken   = os.Getenv("TELEGRAM_BOT_TOKEN")
	Office             *libreofficekit.Office
	SupportedMimetypes = map[string]bool{
		// Microsoft Office
		"application/msword":                                                        true,
		"application/vnd.ms-excel":                                                  true,
		"application/vnd.ms-powerpoint":                                             true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
		// LibreOffice
		"application/vnd.oasis.opendocument.text":         true,
		"application/vnd.oasis.opendocument.presentation": true,
		"application/vnd.oasis.opendocument.spreadsheet":  true,
	}
)

func ProcessDocument(document *libreofficekit.Document, bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	var (
		isBGRA     bool
		rectangles []image.Rectangle
		partsCount int
	)
	if document.GetTileMode() == libreofficekit.BGRATilemode {
		isBGRA = true
	} else {
		isBGRA = false
	}

	documentType := document.GetType()

	if documentType == libreofficekit.TextDocument {
		rectangles = document.GetPartPageRectangles()
		partsCount = len(rectangles)
	} else {
		parts := document.GetParts()
		partsCount = parts
	}

	for i := 0; i < partsCount; i++ {
		var (
			width      int
			height     int
			tilePosX   int
			tilePosY   int
			tileWidth  int
			tileHeight int
		)
		if documentType == libreofficekit.TextDocument {
			rectangle := rectangles[i]
			width, height = rectangle.Dx(), rectangle.Dy()
			tilePosX = rectangle.Min.X
			tilePosY = rectangle.Min.Y
			tileWidth = rectangle.Dx()
			tileHeight = rectangle.Dy()
		} else {
			document.SetPart(i)
			width, height = document.GetSize()
			tilePosX = 0
			tilePosY = 0
			tileWidth = width
			tileHeight = height
		}
		canvasWidth := libreofficekit.TwipsToPixels(width, PreviewsDPI)
		canvasHeight := libreofficekit.TwipsToPixels(height, PreviewsDPI)

		m := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
		pixels := unsafe.Pointer(&m.Pix[0])

		document.PaintTile(pixels, canvasWidth, canvasHeight, tilePosX, tilePosY, tileWidth, tileHeight)

		log.Println(fmt.Sprintf("[%s] Rendered page #%d", message.Document.FileID, i))

		if isBGRA {
			libreofficekit.BGRA(m.Pix)
		}

		CreateAndSendPNG(i, m, bot, message)
	}
}

func ProcessFile(fileUrl string, bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	log.Println(fmt.Sprintf("[%s] Received file from [%s]: ('%v', '%v', %v bytes).", message.Document.FileID, message.Chat.UserName, message.Document.FileName, message.Document.MimeType, message.Document.FileSize))
	log.Println(fmt.Sprintf("[%s] Downloading file: `%s`.", message.Document.FileID, fileUrl))
	n, tempFilePath := DownloadToTempFile(fileUrl)
	log.Println(fmt.Sprintf("[%s] Saved as `%s`[%d].", message.Document.FileID, tempFilePath, n))
	realMimetype, _ := magicmime.TypeByFile(tempFilePath)
	log.Println(fmt.Sprintf("[%s] Libmagic mimetype: `%s`.", message.Document.FileID, realMimetype))
	if !SupportedMimetypes[realMimetype] {
		SendReply(bot, message, UnsupportedMimetypeReply)
		return
	} else {
		SendReply(bot, message, DocumentDownloadedReply)
	}
	defer os.Remove(tempFilePath)
	if n == message.Document.FileSize {
		Office.Mutex.Lock()
		log.Println(fmt.Sprintf("[%s] Locked LibreOfficeKit.", message.Document.FileID))
		document, err := Office.LoadDocument(tempFilePath)
		log.Println(fmt.Sprintf("[%s] LibreOffice document type: [%d].", message.Document.FileID, document.GetType()))
		defer document.Close()
		if err == nil {
			document.InitializeForRendering("")
			ProcessDocument(document, bot, message)
		}
		Office.Mutex.Unlock()
		log.Println(fmt.Sprintf("[%s] Unlocked LibreOfficeKit.", message.Document.FileID))
	} else {
		log.Println(fmt.Sprintf("[%s] Corrupt file.", message.Document.FileID))
	}
	SendReply(bot, message, DoneReply)
}

func main() {
	log.Println("Started.")

	var err error

	Office, err = libreofficekit.NewOffice(LibreOfficePath)
	if err != nil {
		log.Panic(err)
	} else {
		log.Println("Loaded LibreOfficeKit.")
	}

	err = magicmime.Open(magicmime.MAGIC_MIME_TYPE | magicmime.MAGIC_SYMLINK | magicmime.MAGIC_ERROR)
	if err != nil {
		log.Panic("Failed to load libmagic.")
	} else {
		log.Println("Loaded libmagic.")
	}
	defer magicmime.Close()

	bot, err := tgbotapi.NewBotAPI(TelegramBotToken)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Authorized on account %s.", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		message := update.Message
		if message.Document == nil {
			var reply string
			if message.Text == "/start" {
				reply = StartReply
			} else {
				reply = NotDocumentReply
			}
			SendReply(bot, message, reply)
		} else {
			if SupportedMimetypes[message.Document.MimeType] {
				if message.Document.FileSize > (DownloadFilesizeLimit) {
					SendReply(bot, message, LargeFileReply)
				} else {
					fileUrl, _ := bot.GetFileDirectURL(message.Document.FileID)
					go ProcessFile(fileUrl, bot, message)
				}
			} else {
				log.Println(fmt.Sprintf("[%s] Unknown mimetype: [%s]", message.Document.FileID, message.Document.MimeType))
				SendReply(bot, message, UnsupportedMimetypeReply)
			}
		}
	}
}
