package main

import "log"

func InitLogger() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
