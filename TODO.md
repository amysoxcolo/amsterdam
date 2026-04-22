# Future Directions for Amsterdam

## Immediate Cleanups Required

* <s>A better way to set up the database than `setup/database.sql`. Bring the table setup into the application somehow.
  The [migrate](https://github.com/golang-migrate/migrate) library might be of use here.</s>
* ~~Database format migrations.~~
* Allow use of Postgres as a database.
* Dockerization.
* Implement proper help and online documentation.

## Functions That Were "Not Yet Implemented" In Venice Circa 2006

* Either implement the Calendar and Chat, or take those menu entries out.
  * Should those be community "services" instead?
  * For Chat, if it's implemented, it should use XMPP.

## Additional Items

* Ensure design is responsive enough that we can use the site on mobile devices.
* Decouple from MySQL, introduce other database support. Postgres and SQLite are the two priorities here.
* Fix password storage. Straight SHA-1 hashes aren't gonna cut it. There are better ways.
* Introduce OAuth authentication? (Related to above)
* Post storage - replace "limited" HTML with Markdown. (HTML Checker still required though)
* A proper API for the system.
* Topics as RSS feeds. Later, conferences as RSS feeds.
* Figure out how to interlink Amsterdam instances. ActivityPub in some fashion?
