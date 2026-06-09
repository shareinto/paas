package migrations

import (
	"github.com/shareinto/paas/internal/modules/appenv"
	"github.com/shareinto/paas/internal/modules/audit"
	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/modules/gitops"
	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/notification"
	"github.com/shareinto/paas/internal/modules/sourcerepository"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/platform/database"
)

func All() []database.Migration {
	var out []database.Migration
	for _, migrations := range [][]database.Migration{
		identityaccess.Migrations,
		tenantproject.Migrations,
		sourcerepository.Migrations,
		appenv.Migrations,
		build.Migrations,
		delivery.Migrations,
		audit.Migrations,
		notification.Migrations,
		clusteragent.Migrations,
		gitops.Migrations,
		repositorySnapshotMigrations,
	} {
		out = append(out, migrations...)
	}
	return append([]database.Migration(nil), out...)
}
