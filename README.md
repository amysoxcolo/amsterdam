# The Amsterdam Web Communities System

## Purpose

Amsterdam is a web-based system allowing for the hosting of multiple virtual communities, each with services such as conferencing.
Users on an Amsterdam site may be a member of multiple independent communities, all on the same site.

Amsterdam is intended to become a modern platform styled after the first generation of online communities, designed for
resilience, autonomy, and human-scale interaction.

The first public version of Amsterdam has feature parity with the platform that successfully hosted the original Electric Minds
community from 2000-2006, but rebuilt in a modern environment with updated rendering.  Future versions will extend the functionality
from there.

### Why now?

Communities like Electric Minds were largely supplanted by the major social media sites, which built huge systems for global
interaction at massive scale.  They pushed smaller communities out of existence the way Walmart drove smaller shops out of business
in cities and towns across America.

Now we're seeing what happens as a result:

* Networks that use algorithms to prioritize feeding users ads and propaganda over genuine human interaction.
* The gradual "enshittification" _(to use Cory Doctorow's term)_ of services, first to benefit business customers over users,
  then to benefit shareholders over both.
* Entire established social networks being taken over by billionaires or co-opted by governments to promote their political agendas.
* A loss of trust, safety, agency, and democratic control.

We need _human-scale_ community again.  Amsterdam can be a baseline for bringing that back...because its heritage lies in those
older systems that _worked,_ and sustained _real communities_ in the process. It was built by someone who's _been there,_ who not
only wrote the code, but was an active participant in the community that used it.

### Project Vision & Values

Amsterdam as a project intends to prioritize certain things:

* Human-scale over global scale. Hundreds or thousands of users, not billions.
* Resilience over growth, and _especially_ over growth for growth's sake.
* Many smaller sites, not one big one. These sites should work _together,_ not act as more silos.
* Tools that serve _community members, moderators, and hosts,_ not shareholders.
* Contribution quality over ideology or factionalism.  Contributors of _all backgrounds_ are welcome, with a focus on the quality
  of the final product.

### Why "Amsterdam"?

The first implementation of this concept was in Durand Communications' CommunityWare software, which was code-named "Rome" when it
was in development. Rome was a center of community in the ancient world.

The second implementation was the Java code I wrote to rescue Electric Minds; I named it "Venice," which was a center of community
during the Renaissance.

This new implementation is named "Amsterdam," which was a center of community during the Age of Exploration, in particular, the
Dutch Golden Age.

## Building Amsterdam

From the root of the source tree, just run `go build` to build the `amsterdam` executable.

To regenerate the `tailwind.css` file (located in `ui/static/css`), you will need the Tailwind CSS command-line executable.
Download it from [the Tailwind GitHub](https://github.com/tailwindlabs/tailwindcss/releases/) and install it as `tailwindcss`
in your `PATH`. Then run `go generate` to regenerate the CSS file before you run `go build` to build the executable.

## Installing Amsterdam

You will need a MySQL database to store Amsterdam data. Create a new empty database, then, from the command line, use the command:

> `mysql -u root -p _databasename_ \< setup/mysql-database.sql`

(Replace _databasename_ with the name of your database. If you use a user other than `root` for administrative access to your
MySQL server, use that.)

Ensure a user in your database is granted SELECT, INSERT, UPDATE, and DELETE privileges to all tables in your new database.

The database may be specified to Amsterdam with the following command line options or environment variables:

* Host name: Command line options `-t` or `--database-host`, or environment variable `AMSTERDAM_DATABASE_HOST`
* User name: Command line options `-u` or `--database-user`, or environment variable `AMSTERDAM_DATABASE_USER`
* Password: Command line options `-p` or `--database-password`, or environment variable `AMSTERDAM_DATABASE_PASSWORD`
* Database name: Command line options `-d` or `--database-name`, or environment variable `AMSTERDAM_DATABASE_NAME`

All these options may also be specified via the configuration file (see below).

Amsterdam also requires access to a local SMTP server, as it sends out E-mail messages such as account verification,
password reminders, subscribed posts, and messages from conference or community hosts.  It may be specified to Amsterdam
with the following command line options or environment variables:

