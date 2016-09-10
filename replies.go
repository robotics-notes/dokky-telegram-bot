package main

const (
	StartReply               = "Hi!\nNow, select a document and send it to me."
	NotDocumentReply         = "Please, send me only documents.\nI'm quite busy for all that chats."
	LargeFileReply           = "Sorry, I can't download that document, due to Telegram limits (bots can't download files larger than 20 MB)"
	UnsupportedMimetypeReply = "Sorry, I don't support this file type."
	DocumentDownloadedReply  = "Got it.\nIt may take a while, so please stand by."
	DoneReply                = "Done."
)
