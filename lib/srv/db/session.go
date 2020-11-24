/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package db

import (
	"fmt"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/sirupsen/logrus"
)

// sessionContext contains information about a database session.
type sessionContext struct {
	// id is the unique session id.
	id string
	// db is the database instance information.
	db services.DatabaseServer
	// identity is the identity of the connecting teleport user.
	identity tlsca.Identity
	// checker is the access checker for the identity
	checker services.AccessChecker
	// dbUser is the requested database user.
	dbUser string
	// dbName is the requested database name.
	dbName string
	// log is the logger with session specific fields.
	log logrus.FieldLogger
}

// String returns string representation of the session parameters.
func (c *sessionContext) String() string {
	return fmt.Sprintf("db[%v] identity[%v] dbUser[%v] dbName[%v]",
		c.db.GetDatabaseName(), c.identity.Username, c.dbUser, c.dbName)
}
