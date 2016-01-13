package main

import (
	"bufio"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

func pollDb() {

	for {
		time.Sleep((1000 * time.Millisecond) * 60)
		fmt.Println("Hi there from a go routine")
	}
}

func handleReminderRequest(text string) string {
	retMessage := ""

	// Okay, we have a request, so we need to figure out when they want to be reminded
	// and what the message is. The format should be:
	//		!reminder <amount> <message>
	// so we're looking for two blocks after !remind, the amount and the message (which can
	// have spaces, so it'll be whatever it is until the end of the line)

	messageParts := strings.Fields(text)

	// Do we have the right amount of parts?
	if len(messageParts) == 1 {
		return "Not enough parts"
	}

	reminderAmount := messageParts[1]
	reminderPart := strings.Join(append(messageParts[2:]), " ")

	if len(reminderPart) == 0 {
		return "You didn't give me anything to remind you of"
	}

	log.Printf("Amount: %s, Message: %s\n", reminderAmount, reminderPart)

	return retMessage
}
func main() {
	log.Println("*** FUTUREBOT STARTING ***")

	log.Println("Opening the database...")
	db, err := sql.Open("sqlite3", "./futurebot.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	go pollDb()

	// The pattern we're going to use is something like:
	//		!remind 24h Hey! 24 hours ago you asked me to remind you of this!
	// Where:
	//		* !remind is the command we're looking for
	//		* 24h is the amount of time in the future to be reminded
	//			- Valid amounts are h (hours), d (days), w (weeks), m (minutes), s (seconds)
	//		* The text you want messaged to you at that time

	// Temporary - read from stdin as if IRC
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Faux IRC: ")
		text, _ := reader.ReadString('\n')
		fmt.Println(strings.TrimSpace(text))

		// Is this a command to us?
		if utf8.RuneCountInString(text) > 6 && text[0:7] == "!remind" {
			log.Println("Someone wants to be reminded of something...")
			// Try to handle what they entered; if we could handle it in
			// the function, we'll return something like 'okay...' and when
			// we couldn't handle it, we'll return an error to the user
			fmt.Println(handleReminderRequest(text))
		}
	}

	log.Println("*** FUTUREBOT ENDING ***")
}
