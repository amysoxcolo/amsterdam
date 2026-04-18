# The Amsterdam Web Communities System

A self-hosted platform for running _thoughtful, human-scale_ online communities.

Amsterdam is the third generation of community software originally written to keep the Electric Minds community alive after
its original platform shut down.

Its predecessor, the Venice Web Communities System, was created in 2000 to rescue the Electric Minds community after the
shutdown of its original platform. That system kept the community alive until 2006.

Amsterdam is a modern re-implementation of those concepts, designed to support new human-scale communities while preserving
the history of the original one.

**N.B.:** If you are seeing this repository on any site other than `git.erbosoft.com`, it is a mirror. The source of truth
for the project will always be the version of the repository found on `git.erbosoft.com`.

## What Amsterdam Is

Amsterdam is a self-hosted platform for running multiple virtual communities.

It provides:
* Multiple communities hosted on a single site
* A discussion system featuring the linear conferencing model
* Long-form conversation spaces
* User identities shared across communities
* Moderation and community management tools

It is designed for _human-scale communities_ - hundreds or thousands of users, rather than millions.

Amsterdam is intended to become a modern platform styled after the first generation of online communities, designed for
resilience, autonomy, and human-scale interaction.

The first public version of Amsterdam has feature parity with the platform that successfully hosted the original Electric Minds
community from 2000-2006, but rebuilt in a modern environment with updated rendering.  Future versions will extend the functionality
from there.

## What Amsterdam Is Not

Amsterdam is _not_ designed to be:

* A large-scale social media platform
* An algorithmic, feed-driven network.
* A replacement for services like Facebook or X (formerly known as Twitter).
* A mass-broadcast publishing system.

Instead, it focuses on deliberate, human-scale conversation and community identity.

## Live Demo

A live version of Amsterdam may be found at [https://electricminds.org](https://electricminds.org).  This site, "Electric Minds Reborn,"
includes actual community data from Electric Minds circa 2006.

## Why This Exists

Communities from the early days of the Web, like Electric Minds, began as small, independent, and deeply conversational sites.
People gathered in discussion spaces that felt more like shared living rooms than global broadcast platforms.

Over time, most of these communities were largely supplanted by the major social media sites, which displaced many smaller,
community-run spaces by optimizing for a massive, global scale, engagement metrics, and advertising.

Now we're seeing what happens as a result:

* Networks that use algorithms to prioritize feeding users ads and propaganda over genuine human interaction.
* The gradual "enshittification" _(to use Cory Doctorow's term)_ of services, first to benefit business customers over users,
  then to benefit shareholders over both.
* Entire established social networks being taken over by billionaires or co-opted by governments to promote their political agendas.
* A loss of trust, safety, agency, and democratic control.

We need _human-scale_ community again.  Amsterdam can be a baseline for bringing that back...because its heritage lies in those
older systems that _worked,_ and sustained _real communities_ in the process. It was built by someone who's _been there,_ who not
only wrote the code, but was an active participant in the community that used it.

Electric Minds Reborn is both a historical preservation project and a living experiment into whether those ideas can work in a modern Web.

### Project Vision & Values

Amsterdam as a project intends to prioritize certain things:

* Human-scale over global scale. Hundreds or thousands of users, not billions.
* Resilience over growth, and _especially_ over growth for growth's sake.
* Many smaller sites, not one big one. These sites should work _together,_ not act as more silos.
  * Future versions of Amsterdam may support this directly, allowing interaction between independent sites while still providing
    for autonomous, self-hosted communities.
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

## Project status

Amsterdam is in its first (early) public release.  The software is capable of running a full community site.

The project is under active development, and APIs and internal structures may change between releases.

---

## Building Amsterdam

From the root of the source tree, just run `go build` to build the `amsterdam` executable.

To regenerate the `tailwind.css` file (located in `ui/static/css`), you will need the Tailwind CSS command-line executable.
Download it from [the Tailwind GitHub](https://github.com/tailwindlabs/tailwindcss/releases/) and install it as `tailwindcss`
in your `PATH`. Then run `go generate` to regenerate the CSS file before you run `go build` to build the executable.

## Installing Amsterdam

You will need a MySQL database to store Amsterdam data. Create a new empty database.

Ensure a user in your database is granted all privileges to all tables in your new database.
This is the user that you will configure Amsterdam to use.

The database may be specified to Amsterdam with the following command line options or environment variables:

* Host name: Command line options `-t` or `--database-host`, or environment variable `AMSTERDAM_DATABASE_HOST`
* User name: Command line options `-u` or `--database-user`, or environment variable `AMSTERDAM_DATABASE_USER`
* Password: Command line options `-p` or `--database-password`, or environment variable `AMSTERDAM_DATABASE_PASSWORD`
* Database name: Command line options `-d` or `--database-name`, or environment variable `AMSTERDAM_DATABASE_NAME`

All these options may also be specified via the configuration file (see below).

The first time you execute Amsterdam, the necessary database tables will be created and populated.

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

Amsterdam listens on port 1323 by default.  You can change this with the `-l` or `--listen` options on the
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
configuration file, you do not have to specify options for which you do not wish to change the default.

### Initial Login

After starting Amsterdam, you can log into the administrator account, which has the user name "Administrator" with
no password. After logging in, you will be immediately bounced to the profile page, where you _must_ set a password.

## Contributing

See the [Contributors' Guide](CONTRIBUTING.md) for details.

## License

This project is licensed under [the Mozilla Public License, Version 2.0](LICENSE.md).  Its predecessor, Venice,
was licensed under MPL 1.0, so this was a natural choice for the new implementation.
