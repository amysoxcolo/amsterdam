# MySQL script for migrating a Venice database to the Amsterdam schema.
# Written by Amy Bowersox <amy@erbosoft.com>
#---------------------------------------------------------------------------
# Amsterdam Web Communities System
# Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
# 
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#

ALTER TABLE globals RENAME COLUMN max_sig_mbr_page TO max_comm_mbr_page;
ALTER TABLE globals RENAME COLUMN sig_create_lvl TO comm_create_lvl;

ALTER TABLE audit RENAME COLUMN sigid TO commid;
ALTER TABLE audit RENAME INDEX sig_view TO comm_view;

ALTER TABLE contacts RENAME COLUMN owner_sigid TO owner_commid;

ALTER TABLE sigs RENAME COLUMN sigid TO commid;
ALTER TABLE sigs RENAME COLUMN signame TO commname;
ALTER TABLE sigs RENAME AS communities;

ALTER TABLE sigftrs RENAME COLUMN sigid TO commid;
ALTER TABLE sigftrs RENAME AS commftrs;

ALTER TABLE sigmember RENAME COLUMN sigid TO commid;
ALTER TABLE sigmember RENAME AS commmember;

ALTER TABLE sigban RENAME COLUMN sigid TO commid;
ALTER TABLE sigban RENAME AS commban;

ALTER TABLE sigtoconf RENAME COLUMN sigid TO commid;
ALTER TABLE sigtoconf RENAME AS commtoconf;

ALTER TABLE confhotlist RENAME COLUMN sigid TO commid;

ALTER TABLE postpublish RENAME COLUMN sigid TO commid;

ALTER TABLE ipban DROP INDEX by_mask;
ALTER TABLE ipban DROP COLUMN address;
ALTER TABLE ipban DROP COLUMN mask;
ALTER TABLE ipban ADD COLUMN mask_hi BIGINT UNSIGNED NOT NULL AFTER id;
ALTER TABLE ipban ADD COLUMN mask_lo BIGINT UNSIGNED NOT NULL AFTER id;
ALTER TABLE ipban ADD COLUMN address_hi BIGINT UNSIGNED NOT NULL AFTER id;
ALTER TABLE ipban ADD COLUMN address_lo BIGINT UNSIGNED NOT NULL AFTER id;
ALTER TABLE ipban ADD INDEX by_mask (mask_hi, mask_lo);
