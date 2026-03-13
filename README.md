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
This is the user that you will configure Amsterdam to use.

The database may be specified to Amsterdam with the following command line options or environment variables:

* Host name: Command line options `-t` or `--database-host`, or environment variable `AMSTERDAM_DATABASE_HOST`
* User name: Command line options `-u` or `--database-user`, or environment variable `AMSTERDAM_DATABASE_USER`
* Password: Command line options `-p` or `--database-password`, or environment variable `AMSTERDAM_DATABASE_PASSWORD`
* Database name: Command line options `-d` or `--database-name`, or environment variable `AMSTERDAM_DATABASE_NAME`

All these options may also be specified via the configuration file (see below).

Amsterdam also requires access to a local SMTP server, as it sends out E-mail messages such as account verification,
password reminders, subscribed posts, and messages from conference or community hosts.  It may be specified to Amsterdam
with the following command line options or environment variables:

* Host name: Command line options `-m` or `--mail-host`, or environment variable `AMSTERDAM_MAIL_HOST`
* Port number: Command line options `-o` or `--mail-port`, or environment variable `AMSTERDAM_MAIL_PORT`
* TLS option ("none" or "starttls"): `-S` or `--mail-tls`, or environment variable `AMSTERDAM_MAIL_TLS`
* Authentication type ("none" or "plain"): `-a` or `--mail-authtype`, or environment variable `AMSTERDAM_MAIL_AUTHTYPE`
* User name: Command line options `-U` or `--mail-user`, or environment variable `AMSTERDAM_MAIL_USER`
* Password: Command line options `-W` or `--mail-password`, or environment variable `AMSTERDAM_MAIL_PASSWORD`

All these options may also be specified via the configuration file (see below).

By default, Amsterdam sends log messages to stdout. You can change the log level for Amsterdam with the `-L` or `--level`
options on the command line, or the `AMSTERDAM_LOG_LEVEL` environment variable. Valid values are "trace" (most detailed),
"debug", "info", "warn", "error", "fatal", and "panic" (least detailed).

Connect to Amsterdam on port 1323 by default.  You can change this with the `-l` or `--listen` options on the
command line, or with the `AMSTERDAM_LISTEN` environment variable.  Listening addresses are specified as "host:port",
or just ":port" to listen on all interfaces.

By default, Amsterdam runs in debug mode. You can put it in "production" mode by using the `-P` or `--production`
options on the command line, or by setting the `AMSTERDAM_PROD` environment variable.

### The Amsterdam Configuration File

At startup, Amsterdam looks for the configuration file `amsterdam.yaml` in various directories by host OS:

* Unix/Linux: `/usr/local/etc/amsterdam`, `/etc/amsterdam`, `/etc/xdg/amsterdam`
* macOS: `/usr/local/etc/amsterdam`, `/Library/Application Support/Amsterdam`
* Windows: `%PROGRAMDATA%\amsterdam`

You can also specify Amsterdam's configuration file by using the `-C` or `--config` options on the command line,
or with the `AMSTERDAM_CONFIG` environment variable.

The exact format of the configuration file is shown in the `config/default.yaml` file. When creating an Amsterdam
configuration file, you do not have to specify options for which you do not with to change the default.

## Contributing

See the [Contributors' Guide](CONTRIBUTING.md) for details.

## License

This project is licensed under [the Mozilla Public License, Version 2.0](LICENSE.md).

