/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

type ConferenceHotlist struct {
	Uid      int32 `db:"uid"`
	Sequence int16 `db:"sequence"`
	CommId   int32 `db:"commid"`
	ConfId   int32 `db:"confid"`
}
