package server

import "muara.bi/core/internal/repository/pgsql"

type Repository struct {
	UserRepo       pgsql.UserRepositoryProvider
	ConnectionRepo pgsql.ConnectionRepositoryProvider
	SemanticRepo   pgsql.SemanticRepositoryProvider
	ProjectRepo    pgsql.ProjectRepositoryProvider
	DashboardRepo  pgsql.DashboardRepositoryProvider
	WidgetRepo     pgsql.WidgetRepositoryProvider
	ShareRepo      pgsql.ShareRepositoryProvider
	AlertRepo      pgsql.AlertRepositoryProvider
	AIRepo         pgsql.AIRepositoryProvider
}

func NewRepository(internalConnection InternalConnection) Repository {
	return Repository{
		UserRepo:       pgsql.NewUserRepository(internalConnection.DB),
		ConnectionRepo: pgsql.NewConnectionRepository(internalConnection.DB),
		SemanticRepo:   pgsql.NewSemanticRepository(internalConnection.DB),
		ProjectRepo:    pgsql.NewProjectRepository(internalConnection.DB),
		DashboardRepo:  pgsql.NewDashboardRepository(internalConnection.DB),
		WidgetRepo:     pgsql.NewWidgetRepository(internalConnection.DB),
		ShareRepo:      pgsql.NewShareRepository(internalConnection.DB),
		AlertRepo:      pgsql.NewAlertRepository(internalConnection.DB),
		AIRepo:         pgsql.NewAIRepository(internalConnection.DB),
	}
}
