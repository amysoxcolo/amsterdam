# The Amsterdam HTML Checker

One of the key elements to making conferencing work in the Amsterdam system is the HTML Checker, which is applied to every post,
pseud, and topic name that are added to the system.  This component, which lives in the "htmlcheck" package and subdirectory,
is derived from an original CommunityWare ASP ActiveX component (written in C++ with ATL and STL), which was then reimplemented
in Java for Venice, and then reimplemented in Go for Amsterdam.

The component's objective is to balance _safety,_ _expressiveness,_ and _forgiveness._ It ensures that user-generated content
cannot break page layout, while still allowing limited markup and community-specific linking syntax.

## Functions

The HTML Checker takes the raw input text as typed into the post box by a user, and performs the following functions:

1. Only a limited number of HTML tags are permitted to pass through to the output; all others are "escaped out" by turning
   their angle brackets into the appropriate HTML entities.  The exact tags that are allowed through is configurable.
2. Text is word-wrapped to a selectable number of columns, which fits with the preformatted blocks that hold posts.
3. Any HTML tags that are opened and need to be closed, but have not been, are automatically closed.  This prevents
   a user's malformed HTML from affecting the larger site layout.
4. In a "preview" mode, words in the text are spell-checked by matching them against an internal dictionary, and words
   not appearing in the dictionary are highlighted in red.
5. Bare URLs and E-mail addresses appearing in the text are automatically converted into links.  These may also be enclosed
   in angle brackets and converted into links.
6. User names appearing in either angle brackets or parentheses are converted into links to the user's profile.
7. Links to other posts, topics, conferences, or even communities may appear in angle brackets, using an established syntax;
   these are automatically turned into links.

## Configuration

When an instance of the HTML Checker is instantiated with **AmNewHTMLChecker,** the caller specifies a _configuration name_
for the checker to use.  These configurations are expressed in the `configs.yaml` file, which is deserialized at Amsterdam
startup time.  The configuration specifies:

* Basic settings like the word-wrap length and master switches that control the recognition of angle brackets and parentheses.
* Lists of _output filters_ to apply to visible text and "raw" text.
* Lists of _rewriters,_ software components that are applied to various pieces of text:
  * Strings of non-whitespace characters.
  * Individual words.
  * "Tags," text enclosed inside angle brackets.
  * Text enclosed inside parentheses.
* A _tag set_ name, that specifies which HTML tags are allowed through.

## HTML Tag Sets

Individual HTML tags are devided into groups, described as follows:

* _Inline formatting_ tags, such as B, I, EM, and STRONG.
* _Anchors,_ meaning the A tag.
* _Block formatting_ tags, such as P, BR, and BLOCKQUOTE.
* _Active content_ tags, such as EMBED and SCRIPT. These are generally never allowed.
* _Image map_ tags, such as MAP and AREA. These are generally never allowed.
* _Document formatting_ tags like HEAD and BODY, as well as metadata tags like META and LINK. These are generally never allowed.
* _Font format_ tags, which is basically FONT and all the Hx tags.
* _Form_ tags, such as FORM, INPUT, BUTTON, and SELECT. These are generally never allowed.
* _Table_ tags, such as TABLE, TR, and TD.  These are generally never allowed.
* _Change markup_ tags, such as DEL and INS.
* _Frame_ tags such as FRAME, IFRAME, and FRAMESET. These are generally never allowed.
* _Image_ tags, basically the IMG tag.
* _Preformatting_ tags, such as PRE and PLAINTEXT. These are generally never allowed.
* A number of groups of Netscape-specific and Microsoft-specific tags, such as Netscape's (infamous) BLINK tag and Microsoft's
  (equally infamous) MARQUEE and BGSOUND tags. These are generally never allowed.
* Certain tags used in server-side and Java server-side markup. These are generally never allowed.

These groups are further aggregated into _tag sets._ The `normal` tag set consists of the following groups:

* Inline formatting
* Anchor
* Block format
* Font format
* Images

The `restricted` tag set consists of only the "inline formatting" group. It is intended for post pseuds and topic names.

## Rewriters

Rewriters are components identified by registered names that examine a chunk of text, decide if it can be rewritten, and,
if so, apply markup before and after it as necessary.  Examples of rewriters configured in the HTML checker are:

* `emoticon` - Takes certain character pattrns and rewrites them as emoji.  The patterns it recognizes are configured in the
  `emoticons.yaml` file.
* `emoticon_tag` - A variant of the above used inside tag text.
* `email` - Recognizes an E-mail address and creates a `mailto:` link to it.
* `url` - Recognize a URL and create a link surrounding it.
* `postlink` - Recognize a post link and creates a link to it.  (The link is created with an `x-postlink:` schema, which is
  further fixed up when the post is displayed.)
* `userlink` - Recognize a username and create a link to that user's profile.  (The link is created with an `x-userlink:`
  schema, which is further fixed up when the post is displayed.)

## Post Links

Post links have a specific syntax, which was originated on The WELL and implemented in WellEngaged before being reimplemented
in CommunityWare, Venice, and Ansterdam.  Post links are always enclosed in angle brackets.

Here are the various forms of post links supported:

* `<45>` - Link to a single post by number, within the current topic.
* `<13-17>` - Link to a range of posts by number, within the current topic.
* `<64->` - Link to all posts in the current topic, starting at the specified post number.
* `<16.>` - Link to another topic by number, within the current conference. The trailing "." is required.
* `<8.101>` - Link to a single post by number, in a topic by number, in the current conference. (The "range" syntaxes
  for the post number are also supported.)
* `<Commons.>` - Link to another conference by "alias" within the same community. The trailing "." is required.
* `<Altered.6>` - Link to another topic by number, in a conference within the same community.
* `<TechSide.8.14>` - Link to a single post by number, in a topic by number, in a conference within the same community.
  (The "range" syntaxes for the post number are also supported.)
* `<minds!>` - Link to another community by "alias." The trailing "!" is required.
* `<minds!Commons>` Link to another conference in another community.
* `<minds!Altered.9>` Link to another topic by number, in a conference in another community.
* `<minds!TechSide.8.14>` - Link to a single post by number, in a topic by number, in a conference in another community.
  (The "range" syntaxes for the post number are also supported.)

Any of the post link types that are "fully qualified" (that is, start with a specific community alias) can be concatenated to
the special Amsterdam `/go/` URL to jump to the specified community, conference, topic, or post(s). This is, in fact, how
`x-postlink:` URLs are resolved at display time.

## Internal operation

The HTML Checker employs a finite-state machine examining the input text one byte at a time. Characters in the "current"
state are saved in a temporary buffer before being written to the main output buffer when the state changes, possibly
after having been modified by a rewriter.

The `context.Context` value passed to **AmNewHTMLChecker** is checked on every iteration of the main parse loop. If
it signals "done," the parser is stopped, allowing the HTML checker to respect external timeouts.  (The value is
stored in the HTML Checker itself, which is generally frowned upon, but used in this case to simplify the external
API since HTML Checker objects are typically scoped to a single request.)

Currently-open tags are managed on an internal stack, which supports the special operation of "remove most recent,"
which searches the stack from the top down for a specific data element and removes it.  This is required because
HTML tags need not be strictly nested.
