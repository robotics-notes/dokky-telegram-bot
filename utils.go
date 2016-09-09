package main

import (
	"fmt"
	"gopkg.in/telegram-bot-api.v4"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

func DownloadToTempFile(fileUrl string) (int, string) {
	tempFile, _ := ioutil.TempFile(os.TempDir(), fmt.Sprintf(TempfilePrefix))
	response, _ := http.Get(fileUrl)
	n, _ := io.Copy(tempFile, response.Body)
	defer response.Body.Close()
	defer tempFile.Close()
	return int(n), tempFile.Name()
}

func CreateAndSendPNG(page int, image *image.RGBA, bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	tempPreviewFile, _ := ioutil.TempFile(os.TempDir(), fmt.Sprintf(TempPreviewPrefix, message.Document.FileID, page))
	png.Encode(tempPreviewFile, image)
	tempPreviewFile.Close()
	validPreviewFileName := fmt.Sprintf("%s_%d.png", tempPreviewFile.Name(), page+1)
	os.Rename(tempPreviewFile.Name(), validPreviewFileName)
	msg := tgbotapi.NewPhotoUpload(message.Chat.ID, validPreviewFileName)
	msg.ReplyToMessageID = message.MessageID
	msg.Caption = fmt.Sprintf("Page #%d", page+1)
	bot.Send(msg)
	os.Remove(validPreviewFileName)
}

func SendReply(bot *tgbotapi.BotAPI, message *tgbotapi.Message, reply string) {
	msg := tgbotapi.NewMessage(message.Chat.ID, reply)
	msg.ReplyToMessageID = message.MessageID
	bot.Send(msg)
}
