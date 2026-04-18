# Future Directions for Amsterdam

After the point where it reaches feature parity with Venice circa 2006.

## Pre-Launch Polish

* ~~Policy page support.~~
* ~~User agreement in a separate file rather than directly in settings.~~
* ~~Support all customizations that were done with the EMinds instance of Venice.~~
* ~~Gitea-like status page showing Go-specific internals.~~
* ~~Build static Tailwind CSS file rather than using remote-loaded version. (Gate on debug/prod flag)~~
* ~~Rate limiter.~~
* ~~Better logging configuration.~~

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

## Architectural Goofs

* Conference Aliases are effectively in a system-wide namespace. Should be per-community.

## Additional Items

* Decouple from MySQL, introduce other database support. Postgres and SQLite are the two priorities here.
* Fix password storage. Straight SHA-1 hashes aren't gonna cut it. There are better ways.
* Introduce OAuth authentication? (Related to above)
* Post storage - replace "limited" HTML with Markdown. (HTML Checker still required though)
* A proper API for the system.
* Topics as RSS feeds. Later, conferences as RSS feeds.
* Figure out how to interlink Amsterdam instances. ActivityPub in some fashion?
