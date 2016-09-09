package main

import (
	"fmt"
	"github.com/docsbox/go-libreofficekit"
	"gopkg.in/telegram-bot-api.v4"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"unsafe"
)

var (
	Office             *libreofficekit.Office
	Bot                *tgbotapi.BotAPI
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

const (
	LibreOfficePath   = "/usr/lib/libreoffice/program/"
	TempfilePrefix    = "dokky-file-"
	TempPreviewPrefix = "dokky-file-%s-%d"
	PreviewsDPI       = 75
)

func DownloadToTempFile(fileUrl string) (int, string) {
	tempFile, _ := ioutil.TempFile(os.TempDir(), fmt.Sprintf(TempfilePrefix))
	response, _ := http.Get(fileUrl)
	n, _ := io.Copy(tempFile, response.Body)
	defer response.Body.Close()
	defer tempFile.Close()
	return int(n), tempFile.Name()
}

func CreateAndSendPNG(page int, image *image.RGBA, message *tgbotapi.Message) {
	tempPreviewFile, _ := ioutil.TempFile(os.TempDir(), fmt.Sprintf(TempPreviewPrefix, message.Document.FileID, page))
	png.Encode(tempPreviewFile, image)
	tempPreviewFile.Close()
	validPreviewFileName := fmt.Sprintf("%s_%d.png", tempPreviewFile.Name(), page+1)
	os.Rename(tempPreviewFile.Name(), validPreviewFileName)
	msg := tgbotapi.NewPhotoUpload(message.Chat.ID, validPreviewFileName)
	msg.ReplyToMessageID = message.MessageID
	msg.Caption = fmt.Sprintf("Page #%d", page+1)
	Bot.Send(msg)
	os.Remove(validPreviewFileName)
}

func ProcessDocument(document *libreofficekit.Document, message *tgbotapi.Message) {
	var (
		isBGRA        bool
		rectangles    []image.Rectangle
		partsCount    int
		width, height int
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
		width, height = rectangles[0].Dx(), rectangles[0].Dy()
	} else {
		parts := document.GetParts()
		partsCount = parts
		width, height = document.GetSize()
	}

	canvasWidth := libreofficekit.TwipsToPixels(width, PreviewsDPI)
	canvasHeight := libreofficekit.TwipsToPixels(height, PreviewsDPI)

	m := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	pixels := unsafe.Pointer(&m.Pix[0])

	for i := 0; i < partsCount; i++ {
		if documentType == libreofficekit.TextDocument {
			rectangle := rectangles[i]
			document.PaintTile(pixels, canvasWidth, canvasHeight, rectangle.Min.X, rectangle.Min.Y, rectangle.Dx(), rectangle.Dy())
		} else {
			document.SetPart(i)
			document.PaintTile(pixels, canvasWidth, canvasHeight, 0, 0, width, height)
		}
		log.Println(fmt.Sprintf("[%s] Rendered page #%d", message.Document.FileID, i))
		if isBGRA {
			libreofficekit.BGRA(m.Pix)
		}
		CreateAndSendPNG(i, m, message)
	}
}

func ProcessFile(fileUrl string, message *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Got it.\nIt may take a while, so please stand by.")
	msg.ReplyToMessageID = message.MessageID
	Bot.Send(msg)
	log.Println(fmt.Sprintf("[%s] Received file from [%s]: ('%v', '%v', %v bytes).", message.Document.FileID, message.Chat.UserName, message.Document.FileName, message.Document.MimeType, message.Document.FileSize))
	log.Println(fmt.Sprintf("[%s] Downloading file: `%s`.", message.Document.FileID, fileUrl))
	n, tempFilePath := DownloadToTempFile(fileUrl)
	log.Println(fmt.Sprintf("[%s] Saved as `%s`[%d].", message.Document.FileID, tempFilePath, n))
	defer os.Remove(tempFilePath)
	if n == message.Document.FileSize {
		Office.Mutex.Lock()
		log.Println(fmt.Sprintf("[%s] Locked LibreOfficeKit.", message.Document.FileID))
		document, err := Office.LoadDocument(tempFilePath)
		log.Println(fmt.Sprintf("[%s] LibreOffice document type: [%d].", message.Document.FileID, document.GetType()))
		defer document.Close()
		if err == nil {
			document.InitializeForRendering("")
			ProcessDocument(document, message)
		}
		Office.Mutex.Unlock()
		log.Println(fmt.Sprintf("[%s] Unlocked LibreOfficeKit.", message.Document.FileID))
	} else {
		log.Println(fmt.Sprintf("[%s] Corrupt file.", message.Document.FileID))
	}
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
	defer Office.Close()

	Bot, err = tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Authorized on account %s.", Bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := Bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		message := update.Message
		if message.Document == nil {
			var reply string
			if message.Text == "/start" {
				reply = "Hi!\nNow, select a document and send it to me."
			} else {
				reply = "Please, send me only documents.\n" +
					"I'm quite busy for all that friendy-chats."
			}
			msg := tgbotapi.NewMessage(message.Chat.ID, reply)
			msg.ReplyToMessageID = message.MessageID
			Bot.Send(msg)
		} else {
			if SupportedMimetypes[message.Document.MimeType] {
				if message.Document.FileSize > (1024 * 1024 * 20) {
					msg := tgbotapi.NewMessage(message.Chat.ID, "Sorry, I can't download that document, due to Telegram limits (bots can't download files larger than 20 MB)")
					msg.ReplyToMessageID = message.MessageID
					Bot.Send(msg)
				} else {
					fileUrl, _ := Bot.GetFileDirectURL(message.Document.FileID)
					ProcessFile(fileUrl, message)
				}
			} else {
				log.Println(fmt.Sprintf("[%s] Unknown mimetype: [%s]", message.Document.FileID, message.Document.MimeType))
				reply := "Sorry, I don't support this file type."
				msg := tgbotapi.NewMessage(message.Chat.ID, reply)
				msg.ReplyToMessageID = message.MessageID
				Bot.Send(msg)
			}
		}
	}
}
