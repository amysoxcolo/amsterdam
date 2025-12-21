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

import "context"

/* AmIsEmailAddressBanned returns true if the given E-mail address is on the "banned" list.
 * Parameters:
 *     ctx - Standard Go context value.
 *     address - The E-mail address to be checked.
 * Returns:
 *     true if the address is banned, false if not.
 *     Standard Go error status.
 */
func AmIsEmailAddressBanned(ctx context.Context, address string) (bool, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT by_uid FROM emailban WHERE address = ?", address)
	if err != nil {
		return false, err
	}
	return rs.Next(), nil
}
