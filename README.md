# FutureMessageBot
IRC bot to remind users of something in the future. Syntax is:

`!remind <number><m/d/w/y> message`

Where:
* m = minutes
* d = days
* w = weeks
* y = years (years? really?)

The entry will be stored in the sqlite database and every minute a goroutine will query the table to see what events are older than *right now*, and if there are any, broadcast them in the chat room with the user's nickname (so to kick in personal notification routines in IRC clients). The entry will then be deleted from the table so as to not be broadcast again.


