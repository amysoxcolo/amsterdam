# MySQL script for initializing the Amsterdam database.
# Written by Amy Bowersox <amy@erbosoft.com>
#---------------------------------------------------------------------------
# Amsterdam Web Communities System
# Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
# 
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#

##############################################################################
# Database Creation
##############################################################################

DROP DATABASE IF EXISTS amsterdam;
CREATE DATABASE amsterdam;
USE amsterdam;

##############################################################################
# Table Creation
##############################################################################

# The global parameters table.  This is used for stuff that a Venice admin would be
# likely to edit "on the fly."  Stuff that can only be updated with a shutdown should go
# in the XML config file.  This table has ONLY ONE ROW!
CREATE TABLE globals (
    posts_per_page INT NOT NULL,
    old_posts_at_top INT NOT NULL,
    max_search_page INT NOT NULL,
    max_comm_mbr_page INT NOT NULL,
    max_conf_mbr_page INT NOT NULL,
    fp_posts INT NOT NULL,
    num_audit_page INT NOT NULL,
    comm_create_lvl INT NOT NULL
);

# The global properties table.  The "ndx" parameter is used to indicate what
# element is being loaded, and then the "data" element is parsed.
CREATE TABLE propglobal (
    ndx INT NOT NULL PRIMARY KEY,
    data VARCHAR(255)
);

# The audit records table.  Most "major" events add a record to this table.
CREATE TABLE audit (
    record BIGINT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    on_date DATETIME NOT NULL,
    event INT NOT NULL,
    uid INT NOT NULL,
    commid INT NOT NULL DEFAULT 0,
    ip VARCHAR(48),
    data1 VARCHAR(128),
    data2 VARCHAR(128),
    data3 VARCHAR(128),
    data4 VARCHAR(128),
    INDEX on_date_x (on_date),
    INDEX comm_view (commid, on_date)
);

# The user information table.
CREATE TABLE users (
    uid INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    passhash VARCHAR(64) NOT NULL,
    tokenauth VARCHAR(64),
    contactid INT DEFAULT -1,
    is_anon TINYINT DEFAULT 0,
    verify_email TINYINT DEFAULT 0,
    lockout TINYINT DEFAULT 0,
    access_tries SMALLINT DEFAULT 0,
    email_confnum INT DEFAULT 0,
    base_lvl SMALLINT UNSIGNED NOT NULL,
    created DATETIME NOT NULL,
    lastaccess DATETIME,
    passreminder VARCHAR(255) DEFAULT '',
    description VARCHAR(255),
    dob DATE,
    UNIQUE INDEX username_x (username)
);

# User preferences table.
CREATE TABLE userprefs (
    uid INT NOT NULL PRIMARY KEY,
    tzid VARCHAR(64) DEFAULT 'UTC',
    localeid VARCHAR(64) DEFAULT 'en_US'
);

# The per-user properties table.  The "ndx" parameter is used to indicate what
# element is being loaded, and then the "data" element is parsed.
CREATE TABLE propuser (
    uid INT NOT NULL,
    ndx INT NOT NULL,
    data VARCHAR(255),
    PRIMARY KEY (uid, ndx)
);

# Indicates what the top-level "sidebox" configuration is for any given user.
CREATE TABLE sideboxes (
    uid INT NOT NULL,
    boxid INT NOT NULL,
    sequence INT NOT NULL,
    param VARCHAR(255),
    UNIQUE INDEX userboxes (uid, boxid),
    INDEX inorder (uid, sequence)
);

# The contact information table.  This is used for both users and communities.
CREATE TABLE contacts (
    contactid INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    given_name VARCHAR(64),
    family_name VARCHAR(64),
    middle_init CHAR(1),
    prefix VARCHAR(8),
    suffix VARCHAR(16),
    company VARCHAR(255),
    addr1 VARCHAR(255),
    addr2 VARCHAR(255),
    locality VARCHAR(64),
    region VARCHAR(64),
    pcode VARCHAR(16),
    country CHAR(2),
    phone VARCHAR(32),
    fax VARCHAR(32),
    mobile VARCHAR(32),
    email VARCHAR(255),
    pvt_addr TINYINT DEFAULT 0,
    pvt_phone TINYINT DEFAULT 0,
    pvt_fax TINYINT DEFAULT 0,
    pvt_email TINYINT DEFAULT 0,
    owner_uid INT NOT NULL,
    owner_commid INT DEFAULT -1,
    photo_url VARCHAR(255),
    url VARCHAR(255),
    lastupdate DATETIME
);

# A table listing email addresses which are barred from registering.
CREATE TABLE emailban (
    address VARCHAR(255) NOT NULL PRIMARY KEY,
    by_uid INT,
    on_date DATETIME
);

# The community table.
CREATE TABLE communities (
    commid INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    createdate DATETIME NOT NULL,
    lastaccess DATETIME,
    lastupdate DATETIME,
    read_lvl SMALLINT UNSIGNED NOT NULL,
    write_lvl SMALLINT UNSIGNED NOT NULL,
    create_lvl SMALLINT UNSIGNED NOT NULL,
    delete_lvl SMALLINT UNSIGNED NOT NULL,
    join_lvl SMALLINT UNSIGNED NOT NULL,
    contactid INT DEFAULT -1,
    host_uid INT,
    catid INT NOT NULL DEFAULT 0,
    hide_dir TINYINT DEFAULT 0,
    hide_search TINYINT DEFAULT 0,
    membersonly TINYINT DEFAULT 1,
    is_admin TINYINT DEFAULT 0,
    init_ftr SMALLINT DEFAULT 0,
    commname VARCHAR(128) NOT NULL,
    language VARCHAR(20),
    synopsis VARCHAR(255),
    rules VARCHAR(255),
    joinkey VARCHAR(64),
    alias VARCHAR(32) NOT NULL,
    UNIQUE INDEX alias_x (alias),
    INDEX list_chron (createdate),
    INDEX list_cat (catid, createdate),
    INDEX list_alpha (catid, commname)
);

# The table mapping category IDs to category names.
CREATE TABLE refcategory (
    catid INT NOT NULL PRIMARY KEY,
    parent INT NOT NULL,
    symlink INT NOT NULL,
    hide_dir TINYINT DEFAULT 0,
    hide_search TINYINT DEFAULT 0,
    name VARCHAR(64) NOT NULL,
    UNIQUE INDEX display (parent, name)
);

# The table mapping communities and their associated features.
CREATE TABLE commftrs (
    commid INT NOT NULL,
    ftr_code SMALLINT NOT NULL,
    PRIMARY KEY (commid, ftr_code)
);

# The table mapping members of a community and their access levels.
CREATE TABLE commmember (
    commid INT NOT NULL,
    uid INT NOT NULL,
    granted_lvl SMALLINT UNSIGNED,
    locked TINYINT DEFAULT 0,
    hidden TINYINT DEFAULT 0,
    PRIMARY KEY (commid, uid)
);

# A table listing users which have been banned from joining a community.
CREATE TABLE commban (
    commid INT NOT NULL,
    uid INT NOT NULL,
    by_uid INT,
    on_date DATETIME,
    PRIMARY KEY (commid, uid)
);

# The community properties table.  The "index" parameter is used to indicate what
# element is being loaded, and then the "data" element is parsed.
CREATE TABLE propcomm (
    cid INT NOT NULL,
    ndx INT NOT NULL,
    data VARCHAR(255),
    PRIMARY KEY (cid, ndx)
);

# The table describing conferences.  Like original CW, confs may be linked to more
# than one community.
CREATE TABLE confs (
    confid INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    createdate DATETIME NOT NULL,
    lastupdate DATETIME,
    read_lvl SMALLINT UNSIGNED NOT NULL,
    post_lvl SMALLINT UNSIGNED NOT NULL,
    create_lvl SMALLINT UNSIGNED NOT NULL,
    hide_lvl SMALLINT UNSIGNED NOT NULL,
    nuke_lvl SMALLINT UNSIGNED NOT NULL,
    change_lvl SMALLINT UNSIGNED NOT NULL,
    delete_lvl SMALLINT UNSIGNED NOT NULL,
    top_topic SMALLINT DEFAULT 0,
    name VARCHAR(128) NOT NULL,
    descr VARCHAR(255),
    icon_url VARCHAR(255),
    color VARCHAR(8),
    INDEX name_x (name)
);

# The table that links communities to conferences.
CREATE TABLE commtoconf (
    commid INT NOT NULL,
    confid INT NOT NULL,
    sequence SMALLINT NOT NULL,
    granted_lvl SMALLINT UNSIGNED DEFAULT 0,
    hide_list TINYINT DEFAULT 0,
    PRIMARY KEY (commid, confid),
    INDEX display_ord (commid, sequence)
);

# The table listing "aliases" to a conference for post-linking purposes.
CREATE TABLE confalias (
    confid INT NOT NULL,
    alias VARCHAR(64) NOT NULL PRIMARY KEY,
    INDEX confid_x (confid)
);

# A "membership" table for conferences used to control access to private conferences
# and grant conference hosting powers.
CREATE TABLE confmember (
    confid INT NOT NULL,
    uid INT NOT NULL,
    granted_lvl SMALLINT UNSIGNED DEFAULT 0,
    PRIMARY KEY (confid, uid)
);

# Holds "saved settings" for a user with respect to a conference.
CREATE TABLE confsettings (
    confid INT NOT NULL,
    uid INT NOT NULL,
    default_pseud VARCHAR(255),
    last_read DATETIME,
    last_post DATETIME,
    PRIMARY KEY (confid, uid)
);

# The "hot list" of conferences for a given user.
CREATE TABLE confhotlist (
    uid INT NOT NULL,
    sequence SMALLINT NOT NULL,
    commid INT NOT NULL,
    confid INT NOT NULL,
    PRIMARY KEY (uid, commid, confid),
    INDEX inorder (uid, sequence)
);

# The conference properties table.  The "index" parameter is used to indicate what
# element is being loaded, and then the "data" element is parsed.
CREATE TABLE propconf (
    confid INT NOT NULL,
    ndx INT NOT NULL,
    data VARCHAR(255),
    PRIMARY KEY (confid, ndx)
);

# The conference custom HTML block table.  There are two custom blocks, one at the
# top of the page, one at the bottom, each a maximum of 64K in length.
CREATE TABLE confcustom (
    confid INT NOT NULL PRIMARY KEY,
    htmltop TEXT,
    htmlbottom TEXT
);

# The table describing topics within a conference.
CREATE TABLE topics (
    topicid INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    confid INT NOT NULL,
    num SMALLINT NOT NULL,
    creator_uid INT NOT NULL,
    top_message INT DEFAULT 0,
    frozen TINYINT DEFAULT 0,
    archived TINYINT DEFAULT 0,
    sticky TINYINT DEFAULT 0,
    createdate DATETIME NOT NULL,
    lastupdate DATETIME NOT NULL,
    name VARCHAR(128) NOT NULL,
    UNIQUE INDEX by_num (confid, num),
    UNIQUE INDEX by_name (confid, name),
    INDEX by_date (confid, lastupdate)
);

# Holds "saved settings" for a user with respect to a topic.
CREATE TABLE topicsettings (
    topicid INT NOT NULL,
    uid INT NOT NULL,
    hidden TINYINT DEFAULT 0,
    last_message INT DEFAULT -1,
    last_read DATETIME,
    last_post DATETIME,
    subscribe TINYINT DEFAULT 0,
    PRIMARY KEY (topicid, uid)
);

# The "bozo filter" list for a topic, for use by users in filtering out
# the rantings of other users who are bozos.
CREATE TABLE topicbozo (
    topicid INT NOT NULL,
    uid INT NOT NULL,
    bozo_uid INT NOT NULL,
    PRIMARY KEY (topicid, uid, bozo_uid)
);

# The "header" for a posted message.
CREATE TABLE posts (
    postid BIGINT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    parent BIGINT NOT NULL DEFAULT 0,
    topicid INT NOT NULL,
    num INT NOT NULL,
    linecount INT,
    creator_uid INT NOT NULL,
    posted DATETIME NOT NULL,
    hidden TINYINT DEFAULT 0,
    scribble_uid INT,
    scribble_date DATETIME,
    pseud VARCHAR(255),
    UNIQUE INDEX read_order (topicid, num),
    INDEX date_order (topicid, posted),
    INDEX child_order (parent, num)
);

# The actual message text.
CREATE TABLE postdata (
    postid BIGINT NOT NULL PRIMARY KEY,
    data MEDIUMTEXT,
    FULLTEXT INDEX searchpost (data)
);

# Message attachment.
CREATE TABLE postattach (
    postid BIGINT NOT NULL PRIMARY KEY,
    datalen INT,
    hits INT DEFAULT 0,
    last_hit DATETIME,
    stgmethod SMALLINT DEFAULT 0,
    priority SMALLINT DEFAULT 0,
    filename VARCHAR(255),
    mimetype VARCHAR(128),
    data MEDIUMBLOB
);

# "Bookmark" table for posts we like.
CREATE TABLE postdogear (
    uid INT NOT NULL,
    postid BIGINT NOT NULL,
    PRIMARY KEY (uid, postid)
);

# "Front page" publishing table.
CREATE TABLE postpublish (
    commid INT NOT NULL,
    postid BIGINT NOT NULL PRIMARY KEY,
    by_uid INT NOT NULL,
    on_date DATETIME NOT NULL,
    INDEX display_order (on_date, postid)
);

# Advertisement (actually quote, for now) banners
CREATE TABLE adverts (
    adid INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    imagepath VARCHAR(255) NOT NULL,
    pathstyle SMALLINT NOT NULL DEFAULT 0,
    caption VARCHAR(255),
    linkurl VARCHAR(255)    
);

# Storage space for uploaded images.
CREATE TABLE imagestore (
    imgid INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    typecode SMALLINT DEFAULT 0,
    ownerid INT,
    mimetype VARCHAR(128) NOT NULL,
    length INT NOT NULL,
    data MEDIUMBLOB
);

# Table listing IP addresses that are banned from logging in or registering.
CREATE TABLE ipban (
    id INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
    address_lo BIGINT UNSIGNED NOT NULL,
    address_hi BIGINT UNSIGNED NOT NULL,
    mask_lo BIGINT UNSIGNED NOT NULL,
    mask_hi BIGINT UNSIGNED NOT NULL,
    enable TINYINT NOT NULL DEFAULT 1,
    expire DATETIME,
    message VARCHAR(255) NOT NULL,
    block_by INT NOT NULL,
    block_on DATETIME NOT NULL,
    INDEX by_mask (mask_hi, mask_lo),
    INDEX by_date (block_on)
);

##############################################################################
# Set table access rights
##############################################################################

CREATE USER IF NOT EXISTS 'amsdb'@'localhost'
    IDENTIFIED WITH mysql_native_password BY 'x00yes2k';

GRANT INSERT, DELETE, UPDATE, SELECT, LOCK TABLES ON amsterdam.*
    TO 'amsdb'@'localhost';

##############################################################################
# Constant Data Population
##############################################################################

# Populate the Category table.
# Source: Mozilla Open Directory Project categorization system <http://dmoz.org>;
# additional categorization from WebbMe categories
INSERT INTO refcategory (catid, parent, symlink, name) VALUES
    (0,    -1,  -1, 'Unclassified'),
    (1,    -1,  -1, 'Arts'),
      (16,   1,  -1, 'Animation'),
      (17,   1, 181, 'Antiques'),
      (18,   1,  -1, 'Architecture'),
      (19,   1,  -1, 'Art History'),
      (20,   1,  -1, 'Body Art'),
      (21,   1,  -1, 'Celebrities'),
      (22,   1,  -1, 'Comics'),
      (23,   1,  -1, 'Crafts'),
      (24,   1,  -1, 'Dance'),
      (25,   1,  -1, 'Design'),
      (26,   1,  -1, 'Education'),
      (27,   1,  -1, 'Entertainment'),
      (28,   1,  -1, 'Graphic Design'),
      (29,   1,  -1, 'Humanities'),
      (30,   1,  -1, 'Illustration'),
      (31,   1,  -1, 'Literature'),
      (32,   1,  -1, 'Movies'),
      (33,   1,  -1, 'Music'),
      (34,   1,  -1, 'Myths and Folktales'),
      (35,   1,  -1, 'Native and Tribal'),
      (36,   1,  -1, 'Performing Arts'),
      (37,   1,  -1, 'Photography'),
      (38,   1,  -1, 'Radio'),
      (39,   1,  -1, 'Television'),
      (40,   1,  -1, 'Theatre'),
      (41,   1,  -1, 'Typography'),
      (42,   1,  -1, 'Video'),
      (43,   1,  -1, 'Visual Arts'),
      (44,   1,  -1, 'Writing'),
    (2,    -1,  -1, 'Business'),
      (45,   2,  -1, 'Accounting'),
      (46,   2,  -1, 'Advertising'),
      (47,   2,  -1, 'Aerospace'),
      (48,   2,  -1, 'Agriculture and Forestry'),
      (49,   2,  -1, 'Apparel'),
      (50,   2,  -1, 'Arts and Entertainment'),
      (51,   2,  -1, 'Associations'),
      (52,   2,  -1, 'Aviation'),
      (53,   2,  -1, 'Business Services'),
      (54,   2, 245, 'Classifieds'),
      (55,   2,  -1, 'Computers'),
      (56,   2,  -1, 'Consulting'),
      (57,   2,  -1, 'Construction and Maintenance'),
      (58,   2,  -1, 'Electronics'),
      (59,   2,  -1, 'Employment'),
      (60,   2,  -1, 'Energy and Utilities'),
      (61,   2,  -1, 'Environmental and Safety'),
      (62,   2,  -1, 'Financial Services'),
      (63,   2,  -1, 'Food and Related Products'),
      (64,   2,  -1, 'Insurance'),
      (65,   2,  -1, 'Internet Services'),
      (66,   2,  -1, 'Investing'),
      (67,   2, 290, 'Law'),
      (68,   2,  -1, 'Management'),
      (69,   2,  -1, 'Manufacturing'),
      (70,   2,  -1, 'Marketing'),
      (71,   2,  -1, 'Mining and Drilling'),
      (72,   2,  -1, 'Printing'),
      (73,   2,  -1, 'Publishing'),
      (74,   2,  -1, 'Real Estate'),
      (75,   2,  -1, 'Retail'),
      (76,   2,  -1, 'Security'),
      (77,   2,  -1, 'Small Business'),
      (78,   2,  -1, 'Taxes'),
      (79,   2,  -1, 'Training and Schools'),
      (80,   2,  -1, 'Telecommunications'),
      (81,   2,  -1, 'Transportation'),
      (82,   2,  -1, 'Venture Capital'),
    (3,    -1,  -1, 'Computers'),
      (83,   3,  -1, 'CAD'),
      (84,   3,  -1, 'Computer Science'),
      (85,   3,  -1, 'Consultants'),
      (86,   3,  -1, 'Data Communications'),
      (87,   3,  -1, 'Desktop Publishing'),
      (88,   3,  -1, 'Education'),
      (89,   3,  -1, 'Ethics'),
      (90,   3,  -1, 'Fonts'),
      (91,   3, 124, 'Games'),
      (92,   3,  -1, 'Graphics'),
      (93,   3,  -1, 'Hacking'),
      (94,   3,  -1, 'Hardware'),
      (95,   3,  -1, 'History'),
      (96,   3,  -1, 'Internet'),
      (97,   3,  -1, 'Multimedia'),
      (98,   3,  -1, 'Open Source'),
      (99,   3,  -1, 'Operating Systems'),
      (100,  3,  -1, 'Programming'),
      (101,  3,  -1, 'Robotics'),
      (102,  3,  -1, 'Security'),
      (103,  3,  -1, 'Shopping'),
      (104,  3,  -1, 'Software'),
      (105,  3,  -1, 'Systems'),
    (4,    -1,  -1, 'Games'),
      (106,  4,  -1, 'Board Games'),
      (107,  4,  -1, 'Card Games'),
      (108,  4,  -1, 'Coin-op Games'),
      (109,  4, 123, 'Collectible Card Games'),
      (110,  4,  -1, 'Dice Games'),
      (111,  4, 319, 'Fantasy Sports'),
      (112,  4,  -1, 'Gambling'),
      (113,  4,  -1, 'Game Creation Systems'),
      (114,  4,  -1, 'Game Design'),
      (115,  4,  -1, 'Hand Games'),
      (116,  4,  -1, 'Internet Games'),
      (117,  4,  -1, 'Party Games'),
      (118,  4,  -1, 'Puzzles'),
      (119,  4, 270, 'Retailers'),
      (120,  4,  -1, 'Roleplaying Games'),
      (121,  4,  14, 'Sports'),
      (122,  4,  -1, 'Tile Games'),
      (123,  4,  -1, 'Trading Cards'),
      (124,  4,  -1, 'Video Games'),
      (125,  4,  -1, 'Yard and Deck Games'),
    (5,    -1,  -1, 'Health'),
      (126,  5,  -1, 'Aging'),
      (127,  5,  -1, 'Alternative Medicine'),
      (128,  5,  -1, 'Beauty'),
      (129,  5,  -1, 'Children''s Health'),
      (130,  5,  -1, 'Conditions and Diseases'),
      (131,  5,  -1, 'Dentistry'),
      (132,  5, 280, 'Disabilities'),
      (133,  5,  -1, 'Education'),
      (134,  5,  -1, 'Fitness'),
      (135,  5,  64, 'Health Insurance'),
      (136,  5,  -1, 'Medicine'),
      (137,  5,  -1, 'Men''s Health'),
      (138,  5,  -1, 'Mental Health'),
      (139,  5,  -1, 'Nursing'),
      (140,  5,  -1, 'Nutrition'),
      (141,  5,  -1, 'Occupational Health and Safety'),
      (142,  5,  -1, 'Pharmacy'),
      (143,  5,  -1, 'Public Health and Safety'),
      (144,  5,  -1, 'Reproductive Health'),
      (145,  5,  -1, 'Seniors'' Health'),
      (146,  5,  -1, 'Services'),
      (147,  5,  -1, 'Substance Abuse'),
      (148,  5,  -1, 'Teen Health'),
      (149,  5,  -1, 'Women''s Health'),
    (6,    -1,  -1, 'Home'),
      (150,  6,  -1, 'Apartment Living'),
      (151,  6,  -1, 'Cooking'),
      (152,  6,  -1, 'Do-It-Yourself'),
      (153,  6,  -1, 'Emergency Preparation'),
      (154,  6,  -1, 'Entertaining'),
      (155,  6,  -1, 'Family'),
      (156,  6,  -1, 'Gardens'),
      (157,  6,  -1, 'Home Improvement'),
      (158,  6,  -1, 'Homemaking'),
      (363,  6,  -1, 'Homeowners'),
      (159,  6,  -1, 'Kids'),
      (160,  6,  -1, 'Moving and Relocating'),
      (161,  6,  -1, 'Nursery'),
      (162,  6, 207, 'Pets'),
      (163,  6,  -1, 'Personal Finance'),
      (164,  6,  -1, 'Personal Organization'),
      (165,  6,  -1, 'Relatives'),
      (166,  6,  -1, 'Rural Living'),
      (167,  6,  12, 'Shopping'),
      (168,  6,  -1, 'Urban Living'),
    (7,    -1,  -1, 'News'),
      (169,  7,  -1, 'Alternative Media'),
      (170,  7,  -1, 'Columnists'),
      (171,  7,  -1, 'Current Events'),
      (172,  7,  -1, 'Magazines'),
      (173,  7,  -1, 'Media'),
      (174,  7,  -1, 'Newspapers'),
      (175,  7,  -1, 'Online'),
      (176,  7,  -1, 'Politics'),
      (177,  7,  -1, 'Satire'),
      (178,  7,  -1, 'Weather'),
    (8,    -1,  -1, 'Recreation'),
      (179,  8,  -1, 'Air Hockey'),
      (180,  8,  -1, 'Amateur Radio'),
      (181,  8,  -1, 'Antiques'),
      (182,  8,  -1, 'Audio'),
      (183,  8,  -1, 'Aviation'),
      (184,  8,  -1, 'Birdwatching'),
      (185,  8,  -1, 'Boating'),
      (186,  8, 310, 'Bowling'),
      (187,  8,  -1, 'Climbing'),
      (188,  8,  -1, 'Collecting'),
      (189,  8,  23, 'Crafts'),
      (190,  8,  -1, 'Drugs'),
      (191,  8,  -1, 'Food and Drink'),
      (192,  8,   4, 'Games'),
      (193,  8, 156, 'Gardens'),
      (194,  8, 285, 'Genealogy'),
      (195,  8,  -1, 'Guns'),
      (196,  8,  -1, 'Hot Air Ballooning'),
      (197,  8,  -1, 'Humor'),
      (198,  8,  -1, 'Kites'),
      (199,  8,  -1, 'Knives'),
      (200,  8,  -1, 'Living History'),
      (201,  8, 332, 'Martial Arts'),
      (202,  8,  -1, 'Models'),
      (203,  8,  -1, 'Motorcycles'),
      (204,  8,  -1, 'Nudism'),
      (205,  8,  -1, 'Outdoors'),
      (206,  8,  -1, 'Parties'),
      (207,  8,  -1, 'Pets'),
      (208,  8,  -1, 'Roads and Highways'),
      (209,  8,  -1, 'Scouting'),
      (210,  8,  -1, 'Smoking'),
      (211,  8,  14, 'Sports'),
      (212,  8,  -1, 'Theme Parks'),
      (213,  8,  -1, 'Trains and Railroads'),
      (214,  8,  -1, 'Travel'),
    (9,    -1,  -1, 'Reference and Education'),
      (215,  9,  -1, 'Alumni'),
      (216,  9,  -1, 'Colleges and Universities'),
      (217,  9,  -1, 'Continuing Education'),
      (218,  9,  79, 'Corporate Training'),
      (219,  9,  -1, 'Distance Learning'),
      (220,  9,  -1, 'International'),
      (221,  9,  -1, 'K through 12'),
      (222,  9,  -1, 'Libraries'),
      (223,  9,  -1, 'Museums'),
      (224,  9,  -1, 'Special Education'),
      (225,  9,  -1, 'Vocational Education'),
    (10,   -1,  -1, 'Regional'),
      (226, 10,  -1, 'International'),
      (227, 10,  -1, 'US'),
    (11,   -1,  -1, 'Science'),
      (228, 11,  -1, 'Agriculture'),
      (229, 11,  -1, 'Alternative Science'),
      (230, 11,  -1, 'Astronomy'),
      (231, 11,  -1, 'Biology'),
      (232, 11,  -1, 'Chemistry'),
      (233, 11,  -1, 'Earth Sciences'),
      (234, 11,  -1, 'Environment'),
      (235, 11,  -1, 'Mathematics'),
      (236, 11,  -1, 'Physics'),
      (237, 11,  -1, 'Science in Society'),
      (238, 11,  -1, 'Social Sciences'),
      (239, 11,  -1, 'Space'),
      (240, 11,  -1, 'Technology'),
    (12,   -1,  -1, 'Shopping'),
      (241, 12,  -1, 'Antiques and Collectibles'),
      (242, 12,  -1, 'Auctions'),
      (243, 12,  -1, 'Books'),
      (244, 12,  -1, 'Children'),
      (245, 12,  -1, 'Classifieds'),
      (246, 12,  -1, 'Clothing'),
      (247, 12, 103, 'Computers'),
      (248, 12,  -1, 'Consumer Electronics'),
      (249, 12,  -1, 'Crafts'),
      (250, 12,  -1, 'Entertainment'),
      (251, 12,  -1, 'Ethnic and Regional'),
      (252, 12,  -1, 'Flowers'),
      (253, 12,  -1, 'Food and Drink'),
      (254, 12,  -1, 'Furniture'),
      (255, 12,  -1, 'Gifts'),
      (256, 12,  -1, 'Health and Beauty'),
      (257, 12,  -1, 'Holidays'),
      (258, 12,  -1, 'Home and Garden'),
      (259, 12,  -1, 'Jewelry'),
      (260, 12,  -1, 'Music and Video'),
      (261, 12,  -1, 'Niche'),
      (262, 12,  -1, 'Office Products'),
      (263, 12,  -1, 'Pets'),
      (264, 12,  -1, 'Photography'),
      (265, 12,  -1, 'Recreation and Hobbies'),
      (266, 12,  -1, 'Religious'),
      (267, 12,  -1, 'Sports'),
      (268, 12,  -1, 'Tobacco'),
      (269, 12,  -1, 'Tools'),
      (270, 12,  -1, 'Toys and Games'),
      (271, 12,  -1, 'Travel'),
      (272, 12,  -1, 'Vehicles'),
      (273, 12,  -1, 'Visual Arts'),
      (274, 12,  -1, 'Weddings'),
      (275, 12,  -1, 'Wholesale'),
    (13,   -1,  -1, 'Society'),
      (276, 13,  -1, 'Activism'),
      (277, 13,  -1, 'Advice'),
      (278, 13,  -1, 'Crime'),
      (279, 13,  -1, 'Death'),
      (280, 13,  -1, 'Disabled'),
      (281, 13,  -1, 'Ethnicity'),
      (282, 13,  -1, 'Folklore'),
      (283, 13,  -1, 'Future'),
      (284, 13,  -1, 'Gay/Lesbian/Bisexual'),
      (285, 13,  -1, 'Genealogy'),
      (286, 13,  -1, 'Government'),
      (287, 13,  -1, 'History'),
      (288, 13,  -1, 'Holidays'),
      (289, 13,  -1, 'Issues'),
      (290, 13,  -1, 'Law'),
      (291, 13,  -1, 'Lifestyle Choices'),
      (292, 13,  -1, 'Military'),
      (293, 13,  -1, 'Paranormal'),
      (294, 13,  -1, 'People'),
      (295, 13,  -1, 'Philosophy'),
      (296, 13,  -1, 'Politics'),
      (297, 13,  -1, 'Recovery and Support Groups'),
      (298, 13,  -1, 'Relationships'),
      (299, 13,  -1, 'Religion and Spirituality'),
      (300, 13,  -1, 'Sexuality'),
      (301, 13,  -1, 'Subcultures'),
      (302, 13,  -1, 'Transgendered'),
      (303, 13,  -1, 'Work'),
    (14,   -1,  -1, 'Sports'),
      (304, 14,  -1, 'Archery'),
      (305, 14,  -1, 'Badminton'),
      (306, 14,  -1, 'Baseball'),
      (307, 14,  -1, 'Basketball'),
      (308, 14,  -1, 'Billiards'),
      (309, 14,  -1, 'Boomerang'),
      (310, 14,  -1, 'Bowling'),
      (311, 14,  -1, 'Boxing'),
      (312, 14,  -1, 'Cheerleading'),
      (313, 14,  -1, 'Cricket'),
      (314, 14,  -1, 'Croquet'),
      (315, 14,  -1, 'Cycling'),
      (316, 14,  -1, 'Darts'),
      (317, 14,  -1, 'Equestrian'),
      (318, 14,  -1, 'Extreme Sports'),
      (319, 14,  -1, 'Fantasy'),
      (320, 14,  -1, 'Fencing'),
      (321, 14,  -1, 'Fishing'),
      (322, 14,  -1, 'Flying Discs'),
      (323, 14,  -1, 'Football'),
      (324, 14,  -1, 'Golf'),
      (325, 14,  -1, 'Greyhound Racing'),
      (326, 14,  -1, 'Gymnastics'),
      (327, 14,  -1, 'Handball'),
      (328, 14,  -1, 'Hockey'),
      (329, 14,  -1, 'Lacrosse'),
      (330, 14,  -1, 'Laser Games'),
      (331, 14,  -1, 'Lumberjack'),
      (332, 14,  -1, 'Martial Arts'),
      (333, 14,  -1, 'Motor Sports'),
      (334, 14,  -1, 'Orienteering'),
      (335, 14,  -1, 'Paintball'),
      (336, 14,  -1, 'Racquetball'),
      (337, 14,  -1, 'Rodeo'),
      (338, 14,  -1, 'Roller Derby'),
      (339, 14,  -1, 'Rope Skipping'),
      (340, 14,  -1, 'Rugby'),
      (341, 14,  -1, 'Running'),
      (342, 14,  -1, 'Sailing'),
      (343, 14,  -1, 'Shooting'),
      (344, 14, 267, 'Shopping'),
      (345, 14,  -1, 'Skateboarding'),
      (346, 14,  -1, 'Skating'),
      (347, 14,  -1, 'Skiing'),
      (348, 14,  -1, 'Sledding'),
      (349, 14,  -1, 'Sled Dog Racing'),
      (350, 14,  -1, 'Snowboarding'),
      (351, 14,  -1, 'Soccer'),
      (352, 14,  -1, 'Softball'),
      (353, 14,  -1, 'Squash'),
      (354, 14,  -1, 'Strength Sports'),
      (355, 14,  -1, 'Table Tennis'),
      (356, 14,  -1, 'Tennis'),
      (357, 14,  -1, 'Track and Field'),
      (358, 14,  -1, 'Volleyball'),
      (359, 14,  -1, 'Walking'),
      (360, 14,  -1, 'Water Sports'),
      (361, 14,  -1, 'Winter Sports'),
      (362, 14,  -1, 'Wrestling'),
    (15,   -1,  -1, 'System');
### -- LAST IS 363 -- ###

# Make sure the special "System" category is hidden.
UPDATE refcategory SET hide_dir = 1, hide_search = 1 WHERE catid = 15;

# Create the initial advertisements (quotes).
INSERT INTO adverts (imagepath) VALUES
    ('images/ads/Brown.gif'),
    ('images/ads/Caine.gif'),
    ('images/ads/Frost.gif'),
    ('images/ads/Keller.gif'),
    ('images/ads/Letterman.gif'),
    ('images/ads/Pooh.gif'),
    ('images/ads/Shakespeare.gif'),
    ('images/ads/Thomas.gif'),
    ('images/ads/WolinskiTeamwork.gif'),
    ('images/ads/Wonder.gif'),
    ('images/ads/bonaparte.gif'),
    ('images/ads/buscaglia.gif'),
    ('images/ads/dana.gif'),
    ('images/ads/deadpoets.gif'),
    ('images/ads/ford.gif'),
    ('images/ads/karen.gif'),
    ('images/ads/lynett.gif'),
    ('images/ads/mcauliffe.gif'),
    ('images/ads/midler.gif'),
    ('images/ads/sophocles.gif'),
    ('images/ads/talbert.gif'),
    ('images/ads/torvalds.gif'),
    ('images/ads/wonka.gif'),
    ('images/ads/worf.gif');

##############################################################################
# Database Initialization
##############################################################################

# Initialize the system globals table.
INSERT INTO globals (posts_per_page, old_posts_at_top, max_search_page, max_comm_mbr_page, max_conf_mbr_page,
                     fp_posts, num_audit_page, comm_create_lvl)
    VALUES (20, 2, 20, 50, 50, 10, 100, 1000);

# Initialize the global properies table.
INSERT INTO propglobal (ndx, data)
    VALUES (0, '');

# Add the 'Anonymous Honyak' user to the users table.
# (Do 'SELECT * FROM users WHERE is_anon = 1' to retrieve the AC user details.)
# (UID = 1, CONTACTID = 1)
INSERT INTO users (uid, username, passhash, contactid, is_anon, verify_email, base_lvl, created)
    VALUES (1, 'Anonymous_Honyak', '', 1, 1, 1, 100, '2000-12-01 00:00:00');
INSERT INTO userprefs (uid) VALUES (1);
INSERT INTO propuser (uid, ndx, data) VALUES (1, 0, '');
INSERT INTO contacts (contactid, given_name, family_name, locality, region, pcode, country, email, owner_uid)
    VALUES (1, 'Anonymous', 'User', 'Anywhere', '', '', 'US', 'nobody@example.com', 1);

# Provide the default view for Anonymous Honyak.  This view will be copied to all
# new users.
INSERT INTO sideboxes (uid, boxid, sequence, param)
    VALUES (1, 1, 100, NULL), (1, 2, 200, NULL), (1, 3, 300, NULL);
INSERT INTO confhotlist (uid, sequence, commid, confid)
    VALUES (1, 100, 2, 2);

# Add the 'Administrator' user to the users table.
# (UID = 2, CONTACTID = 2)
INSERT INTO users (uid, username, passhash, contactid, verify_email, base_lvl, created)
    VALUES (2, 'Administrator', '', 2, 1, 64999, '2000-12-01 00:00:00');
INSERT INTO userprefs (uid) VALUES (2);
INSERT INTO propuser (uid, ndx, data) VALUES (2, 0, '');
INSERT INTO contacts (contactid, given_name, family_name, locality, region, pcode, country, email, owner_uid)
    VALUES (2, 'System', 'Administrator', 'Anywhere', '', '', 'US', 'root@your.box.com', 2);

# Create the default view for Administrator.
INSERT INTO sideboxes (uid, boxid, sequence, param)
    VALUES (2, 1, 100, NULL), (2, 2, 200, NULL), (2, 3, 300, NULL);
INSERT INTO confhotlist (uid, sequence, commid, confid)
    VALUES (2, 100, 2, 2);

# Add the administration community to the communities table.
# (COMMID = 1, CONTACTID = 3)
INSERT INTO communities (commid, createdate, read_lvl, write_lvl, create_lvl, delete_lvl, join_lvl, contactid,
                  host_uid, catid, hide_dir, hide_search, membersonly, is_admin, init_ftr,
		  commname, language, synopsis, rules, joinkey, alias)
    VALUES (1, '2000-12-01 00:00:00', 63000, 63000, 63000, 65500, 63000, 3, 2, 15, 1, 1, 1, 1, 0,
            'Administration', 'en-US', 'Administrative Community', 'Administrators only!', '',
	    'Admin');
INSERT INTO contacts (contactid, locality, country, owner_uid, owner_commid)
    VALUES (3, 'Anywhere', 'US', 2, 1);
INSERT INTO propcomm (cid, ndx, data) VALUES (1, 0, '');

# Insert the desired features for the 'Administration' community.
INSERT INTO commftrs (commid, ftr_code)
    VALUES (1, 0), (1, 1), (1, 2), (1, 3), (1, 4);

# Make the 'Administrator' user the host of the 'Administration' community.  Also, the Administrator
# cannot unjoin the community.
INSERT INTO commmember (commid, uid, granted_lvl, locked)
    VALUES (1, 2, 58500, 1);

# Insert the "Administrative Notes" conference into the Administration community.
# (CONFID = 1)
INSERT INTO confs (confid, createdate, read_lvl, post_lvl, create_lvl, hide_lvl, nuke_lvl, change_lvl,
                   delete_lvl, top_topic, name, descr)
    VALUES (1, '2000-12-01 00:00:00', 63000, 63000, 63000, 63000, 64999, 64999, 65500, 0,
            'Administrative Notes', 'Used to store notes and discussions between the site administrators.');
INSERT INTO commtoconf (commid, confid, sequence) VALUES (1, 1, 10);
INSERT INTO confalias (confid, alias) VALUES (1, 'Admin_Notes');
INSERT INTO propconf (confid, ndx, data) VALUES (1, 0, '');

# Make the Administrator the host-of-record of the "Administrative Notes" conference.
INSERT INTO confmember (confid, uid, granted_lvl) VALUES (1, 2, 52500);

# Add the 'Coffeehouse' community.  This is the equivalent of the old CommunityWare
# 'Universal Community.'
# (COMMID = 2, CONTACTID = 4)
INSERT INTO communities (commid, createdate, read_lvl, write_lvl, create_lvl, delete_lvl, join_lvl, contactid,
                  host_uid, catid, membersonly, init_ftr, commname, language, synopsis, rules, alias)
    VALUES (2, '2000-12-01 00:00:00', 100, 58000, 58000, 65500, 500, 4, 2, 0, 1, 0, 'Coffeehouse', 'en-US',
            'A gathering place for news and information for all Amsterdam users.',
	    'Like the man said, do unto others as you would have them do unto you.', 'Coffeehouse');
INSERT INTO contacts (contactid, locality, country, owner_uid, owner_commid)
    VALUES (4, 'Anywhere', 'US', 2, 2);
INSERT INTO propcomm (cid, ndx, data) VALUES (2, 0, '');

# Insert the desired features for Coffeehouse.
INSERT INTO commftrs (commid, ftr_code)
    VALUES (2, 0), (2, 1), (2, 3), (2, 4);

# Make 'Anonymous Honyak' a member of Coffeehouse.  This is important because new users will
# have the membership list of Anonymous Honyak copied to their account on signup (but with
# the 'member' access level).
INSERT INTO commmember (commid, uid, granted_lvl, locked, hidden)
    VALUES (2, 1, 100, 1, 1);

# Make the 'Administrator' user the host of Coffeehouse.
INSERT INTO commmember (commid, uid, granted_lvl, locked)
    VALUES (2, 2, 58500, 1);

# Insert the "General Discussion" conference into Coffeehouse.
# (CONFID = 2)
INSERT INTO confs (confid, createdate, read_lvl, post_lvl, create_lvl, hide_lvl, nuke_lvl, change_lvl,
                   delete_lvl, top_topic, name, descr)
    VALUES (2, '2000-12-01 00:00:00', 6500, 6500, 6500, 52500, 52500, 52500, 58000, 0, 'General Discussion',
            'Your place for general discussion about the system and general topics.');
INSERT INTO commtoconf (commid, confid, sequence) VALUES (2, 2, 10);
INSERT INTO confalias (confid, alias) VALUES (2, 'General');
INSERT INTO propconf (confid, ndx, data) VALUES (2, 0, '');

# Make the Administrator the host-of-record of the "General Discussion" conference.
INSERT INTO confmember (confid, uid, granted_lvl) VALUES (2, 2, 52500);

# Insert the "Test Postings" conference into Coffeehouse.
# (CONFID = 3)
INSERT INTO confs (confid, createdate, read_lvl, post_lvl, create_lvl, hide_lvl, nuke_lvl, change_lvl,
                   delete_lvl, top_topic, name, descr)
    VALUES (3, '2000-12-01 00:00:00', 6500, 6500, 6500, 52500, 52500, 52500, 58000, 0, 'Test Postings',
            'Use this conference to test the conferencing system.');
INSERT INTO commtoconf (commid, confid, sequence) VALUES (2, 3, 20);
INSERT INTO confalias (confid, alias) VALUES (3, 'Test');
INSERT INTO propconf (confid, ndx, data) VALUES (3, 0, '');

# Make the Administrator the host-of-record of the "Test Postings" conference.
INSERT INTO confmember (confid, uid, granted_lvl) VALUES (3, 2, 52500);

##############################################################################
# Test Data Insertion
##############################################################################

# User name: 'TestUser' - password: 'Fishmasters'
# (UID=3, CONTACTID=5)
INSERT INTO users (uid, username, passhash, contactid, verify_email, base_lvl, created)
    VALUES (3, 'TestUser', '6BC1E91CF2917BE1AA0D0D1007C28437D3D3AEDF', 5, 1, 1000, '2000-12-01 00:00:00');
INSERT INTO userprefs (uid) VALUES (3);
INSERT INTO propuser (uid, ndx, data) VALUES (3, 0, '');
INSERT INTO contacts (contactid, given_name, family_name, locality, region, pcode, country, email, owner_uid)
    VALUES (5, 'Test', 'User', 'Denver', 'CO', '80231', 'US', 'testuser@example.com', 3);
INSERT INTO sideboxes (uid, boxid, sequence, param)
    VALUES (3, 1, 100, NULL), (3, 2, 200, NULL), (3, 3, 300, NULL);
INSERT INTO confhotlist (uid, sequence, commid, confid)
    VALUES (3, 100, 2, 2);
INSERT INTO commmember (commid, uid, granted_lvl, locked)
    VALUES (2, 3, 6500, 1);

# Community name: 'Ministry of Silly Walks', hosted by Administrator
# (COMMID=3, CONTACTID=6)
INSERT INTO communities (commid, createdate, read_lvl, write_lvl, create_lvl, delete_lvl, join_lvl, contactid,
                  host_uid, catid, membersonly, init_ftr, commname, language, synopsis, rules, alias)
    VALUES (3, '2001-01-15 00:00:00', 6500, 58000, 58000, 58500, 1000, 6, 2, 197, 1, 0,
            'Ministry of Silly Walks', 'en-US', 'A community devoted to enhancing silly walks.',
	    'You must have a silly walk.', 'sillywalk');
INSERT INTO contacts (contactid, locality, country, owner_uid, owner_commid)
    VALUES (6, 'Anywhere', 'US', 2, 3);
INSERT INTO propcomm (cid, ndx, data) VALUES (3, 0, '');
INSERT INTO commftrs (commid, ftr_code)
    VALUES (3, 0), (3, 1), (3, 3), (3, 4);
INSERT INTO commmember (commid, uid, granted_lvl, locked)
    VALUES (3, 2, 58500, 1);

# Community name: 'Bavarian Illuminati', hosted by Administrator, with a join key
# (COMMID=4, CONTACTID=7)
INSERT INTO communities (commid, createdate, read_lvl, write_lvl, create_lvl, delete_lvl, join_lvl, contactid,
                  host_uid, catid, membersonly, init_ftr, commname, language, synopsis, rules, joinkey, alias)
    VALUES (4, '2001-01-15 00:00:00', 6500, 58000, 58000, 58500, 1000, 7, 2, 301, 1, 0,
            'Bavarian Illuminati', 'en-US', 'Devoted to control of the global political structure.',
	    'Evil Geniuses Only!', 'fnord', 'illuminati');
INSERT INTO contacts (contactid, locality, country, owner_uid, owner_commid)
    VALUES (7, 'Anywhere', 'US', 2, 4);
INSERT INTO propcomm (cid, ndx, data) VALUES (4, 0, '');
INSERT INTO commftrs (commid, ftr_code)
    VALUES (4, 0), (4, 1), (4, 3), (4, 4);
INSERT INTO commmember (commid, uid, granted_lvl, locked)
    VALUES (4, 2, 58500, 1);
