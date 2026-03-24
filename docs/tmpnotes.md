# TEMPORARY NOTES

(to be moved elsewhere)

## Amsterdam Identifier Values

Amsterdam identifier values are used for user names, community aliases, and conference aliases, and may be used
for other purposes in the future. A valid Amsterdam ID consists of characters from the following character set:

* Alphanumerics (A-Z, a-z, 0-9)
* Dash (-)
* Underscore (_)
* Tilde (~)
* Asterisk (*)
* Apostrophe (')
* Dollar sign ($)

All characters are represented in the ISO 8859-1 character set, and may be represented with single-byte encoding
in UTF-8.  Also note that all Amsterdam identifiers are case-insensitive.

### Rationale

The character set was defined starting with the list of characters allowable in URL path components ("pchar" as
defined in [RFC 3986](https://www.rfc-editor.org/rfc/rfc3986.txt), section 3.3, page 23), minus the percent-encoded
forms, so that Amsterdam identifiers would be usable as "path information" in a URL.

From here, various characters were excluded:

* The ampersand (&) was excluded because of its possible confusion with a URL parameter separator, and because it requires HTML escaping.
* The at sign (@) was excluded because of possible confusion with E-mail addresses and XMPP identifiers.
* The plus sign (+) was excluded because of possible confusion with a URL-encoded space character.
* The comma (,) was excluded because of its possible interpretation as a separator character.
* The equals sign (=) was excluded because of its possible confusion with a URL parameter/value separator.
* The semicolon (;) was excluded because of its possible interpretation as a separator character.
* The colon (:) was withheld to provide for a possible future "namespace" expansion (as in XML namespaces).
* The parentheses ((, )) were excluded because of possible confusion with user link syntax in conferencing.
* The period (.) was excluded because of possible confusion with post link syntax in conferencing.
* The exclamation point (!) was excluded because of possible confusion with extended post link syntax in conferencing.

The definition of Amsterdam identifiers was taken almost directly from the definition of Venice identifiers in the predecessor project.
