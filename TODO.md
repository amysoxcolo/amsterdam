# Future Directions for Amsterdam

After the point where it reaches feature parity with Venice circa 2006.

## Immediate Cleanups Required

* A better way to set up the database than `setup/database.sql`. Bring the table setup into the application somehow.
  The [migrate](https://github.com/golang-migrate/migrate) library might be of use here.
* Either implement the Calendar and Chat, or take those menu entries out.
  * Should those be community "services" instead?
  * For Chat, if it's implemented, it should use XMPP.
* Implement proper help and online documentation.

## Additional Items

* Decouple from MySQL, introduce other database support. Postgres and SQLite are the two priorities here.
* Fix password storage. Straight SHA-1 hashes aren't gonna cut it. There are better ways.
* Introduce OAuth authentication? (Related to above)
* Post storage - replace "limited" HTML with Markdown.
* A proper API for the system.
* Figure out how to interlink Amsterdam instances. ActivityPub in some fashion?
