package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/thoj/go-ircevent"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var ircCon *irc.Connection
var roomName = "#zzzxxx111xyzzy"

func pollDb() {

	// We're in a goroutine that we want to run forever in the background, opening
	// the database, running the query to see what events have 'expired', and if there
	// are any, we'll print those, then delete the records so we don't have to deal
	// with them again
	for {
		time.Sleep((1000 * time.Millisecond) * 60)

		// If opening the database couldn't happen, just shrug and we'll try again
		// in a minute
		db, err := sql.Open("sqlite3", "./futurebot.db")
		if err != nil {
			log.Println(err)
			continue
		}

		// Database is open from here on out so let's check what we have to send...
		rows, err := db.Query("select id, nickname, remind_message from messages where remind_datetime < ?", int64(time.Now().Unix()))
		if err != nil {
			log.Fatal(err)
		}

		// If there are any messages to send, we want to get the ids so we can delete them
		// afterwards
		var ids []int

		for rows.Next() {
			var id int
			var nickname string
			var remindMessage string
			rows.Scan(&id, &nickname, &remindMessage)
			log.Printf("Going to remind %s of %s (id was %d)", nickname, remindMessage, id)

			ids = append(ids, id)

			// Tell the user what they wanted to hear
			ircCon.Privmsg(roomName, fmt.Sprintf("Hey %s! You wanted me to remind you of: %s!\n", nickname, remindMessage))
		}
		rows.Close()

		// Now delete those messages so we don't send them again
		for _, id := range ids {
			log.Printf("Deleting reminder %d\n", id)
			_, err = db.Exec("delete from messages where id = ?", id)
			if err != nil {
				log.Fatal(err)
			}
		}

		db.Close()
	}
}
func saveReminderToDb(nickname string, epochOffset int64, remindNiceTime, reminderMessage string) (bool, string) {

	db, err := sql.Open("sqlite3", "./futurebot.db")
	if err != nil {
		log.Fatal(err)
		return false, err.Error()
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
		return false, err.Error()
	}
	stmt, err := tx.Prepare("insert into messages(nickname, remind_datetime, remind_datetime_readable, remind_message, datetime_added) values(?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
		return false, err.Error()
	}
	defer stmt.Close()

	_, err = stmt.Exec(nickname, epochOffset, remindNiceTime, reminderMessage, time.Now().Format(time.RFC850))
	if err != nil {
		log.Fatal(err)
		return false, err.Error()
	}
	tx.Commit()

	return true, "OK"
}

func calculateEpochOffset(reminderAmount string) (bool, int64, string, string) {
	// This is the function where we have all the fun of trying
	// to figure out what the user wants, if the user entered the info
	// correctly, and calculate the epoch number from right now

	// First we need to guarantee that the characters up to the last character,
	// exclusive, can be converted to a number, and if not, we kick it back
	amount, err := strconv.Atoi(reminderAmount[0:(len(reminderAmount) - 1)])
	if err != nil {
		return false, 0, "", fmt.Sprintf("Could not convert %s into a number", reminderAmount[0:(len(reminderAmount)-1)])
	}

	var amountConvertedToSeconds int64 = 0

	var resultMsg string = "Calculating the epoch offset resulted in 0"

	// Now we have to figure out what character is at the end. If it's not something
	// we can handle, we'll return a message saying so
	switch reminderAmount[len(reminderAmount)-1] {
	case 's': // seconds
		amountConvertedToSeconds = int64(amount)
	case 'm': // minutes
		amountConvertedToSeconds = int64(amount * 60)
	case 'h': // hours
		amountConvertedToSeconds = int64(amount * 3600)
	case 'd': // days
		amountConvertedToSeconds = int64(amount * 86400)
	case 'w': // weeks
		amountConvertedToSeconds = int64(amount * 604800)
	case 'y': // years
		amountConvertedToSeconds = int64(amount * 31556952)
	default: // ????
		resultMsg = fmt.Sprintf("Could not interpret %s in %s", reminderAmount[len(reminderAmount)-1], reminderAmount)
	}

	// Did we get the value converted to seconds?
	if amountConvertedToSeconds == 0 {
		// Guess not
		return false, 0, "", resultMsg
	}

	// Now get a time structure with the offset time
	epochOffset := int64(time.Now().Unix()) + amountConvertedToSeconds
	t := time.Unix(epochOffset, 0)
	log.Printf("Reminder d/t is %s and the epoch offset is %d\n", t.Format(time.UnixDate), epochOffset)

	return true, epochOffset, t.Format(time.UnixDate), resultMsg
}
func handleReminderRequest(nickname, text string) string {

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
	reminderMessage := strings.Join(append(messageParts[2:]), " ")

	if len(reminderMessage) == 0 {
		return "You didn't give me anything to remind you of"
	}

	log.Printf("Amount: %s, Message: %s\n", reminderAmount, reminderMessage)

	// 1. calcOk is a boolean saying we were able to calculate an offset
	// 2. epochOffset is the number that represents the date and time of the reminder
	// 3. epochOffsetDate is a nicely-formatted date for the user
	// 4. calcErrMsg is the textual error message to be returned to the user if there was a problem
	calcOk, epochOffset, epochOffsetDate, calcErrMsg := calculateEpochOffset(reminderAmount)

	if calcOk == false {
		return calcErrMsg
	}

	// Sweet, so we have a valid reminder time, so let's get that stored in the database
	saveOk, errMessage := saveReminderToDb(nickname, epochOffset, epochOffsetDate, reminderMessage)
	if saveOk == false {
		return fmt.Sprintf("Sorry, couldn't save the reminder because of %s\n", errMessage)
	}

	return fmt.Sprintf("Okay, on %s (epoch %d) I'll remind you of %s\n", epochOffsetDate, epochOffset, reminderMessage)
}

func main() {
	log.Println("*** FUTUREBOT STARTING ***")

	// Get connected to IRC...
	ircCon = irc.IRC("future-msg-bot", "future-msg-bot")
	err := ircCon.Connect("irc.freenode.net:6667")
	if err != nil {
		log.Fatal(err)
	}

	ircCon.AddCallback("001", func(e *irc.Event) {
		ircCon.Join(roomName)
	})

	// This is the function that will handle the actual sending of the reminders
	go pollDb()

	// The pattern we're going to use is something like:
	//		!remind 24h Hey! 24 hours ago you asked me to remind you of this!
	// Where:
	//		* !remind is the command we're looking for
	//		* 24h is the amount of time in the future to be reminded
	//			- Valid amounts are h (hours), d (days), w (weeks), m (minutes), s (seconds)
	//		* The text you want messaged to you at that time

	ircCon.AddCallback("PRIVMSG", func(e *irc.Event) {
		ircMsg := e.Message()
		if utf8.RuneCountInString(ircMsg) > 6 && ircMsg[0:7] == "!remind" {
			log.Println("Someone wants to be reminded of something...")
			// Try to handle what they entered; if we could handle it in
			// the function, we'll return something like 'okay...' and when
			// we couldn't handle it, we'll return an error to the user
			ircCon.Privmsg(roomName, handleReminderRequest(e.Nick, ircMsg))
		}
	})

	ircCon.Loop()

	log.Println("*** FUTUREBOT ENDING ***")
}
