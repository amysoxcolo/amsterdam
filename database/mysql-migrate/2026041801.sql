# Amsterdam Web Communities System
# Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
# 
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# SPDX-License-Identifier: MPL-2.0
#
CREATE TABLE newconfalias (
    commid INT NOT NULL,
    confid INT NOT NULL,
    alias VARCHAR(64) NOT NULL,
    PRIMARY KEY (commid, alias),
    INDEX confid_x (commid, confid)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

INSERT INTO newconfalias (commid, confid, alias)
    SELECT c.commid, c.confid, a.alias FROM commtoconf c, confalias a 
        WHERE c.confid = a.confid;

DROP TABLE confalias;
ALTER TABLE newconfalias RENAME TO confalias;
