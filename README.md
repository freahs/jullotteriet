# jullotteriet

In my family we have this 'secret santa' arrangement for christmas, in which everyone are randomly assigned someone to by a gift for. We used to do this by picking names, but since we're pretty spread out nowadays, we hade to come up with something else. The sollution was a SMS service, since everyone know how to send and recieve SMS.

## Usage
The following commands are available and are executed by (case insensitive) sms:
 * `lista`: Replies with a list of all registered members
 * `avbryt`: Un-register the member with the phone number from which the SMS are sent
 * `jul [NAME]`: Registeres a member with the name `[NAME]`
 * `starta [SECRET]`: Starts the lottery if `[SECRET]` matches the `LOTTERY_SECRET` environmental variable. All members will get an SMS with the name of the person they are supposed to by a gift for.
	
## Environmental variables

 * `PORT`: The port to run the service on
 * `DATABASE_URL`: URL to a postgres database. Must provide full login to db, i.e. `user:pwd@host/db`
 * `LOTTERY_SECRET`: The secret used to start the lottery.

The app uses [Twilio](www.twilio.com) as SMS platform and uses the following variables:
 
 * `TWILIO_ACCOUNT_SID`
 * `TWILIO_AUTH_TOKEN`
 * `TWILIO_NUMBER`
