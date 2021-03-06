package group

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/ovh/cds/engine/api/database"
	"github.com/ovh/cds/engine/log"
	"github.com/ovh/cds/sdk"
)

// DeleteGroupAndDependencies deletes group and all subsequent group_project, pipeline_project
func DeleteGroupAndDependencies(db database.Executer, group *sdk.Group) error {
	err := DeleteGroupUserByGroup(db, group)
	if err != nil {
		log.Warning("deleteGroupAndDependencies: Cannot delete group user %s: %s\n", group.Name, err)
		return err
	}

	err = deleteGroupEnvironmentByGroup(db, group)
	if err != nil {
		log.Warning("deleteGroupAndDependencies: Cannot delete group env %s: %s\n", group.Name, err)
		return err
	}

	err = deleteGroupPipelineByGroup(db, group)
	if err != nil {
		log.Warning("deleteGroupAndDependencies: Cannot delete group pipeline %s: %s\n", group.Name, err)
		return err
	}

	err = deleteGroupApplicationByGroup(db, group)
	if err != nil {
		log.Warning("deleteGroupAndDependencies: Cannot delete group application %s: %s\n", group.Name, err)
		return err
	}

	err = deleteGroupProjectByGroup(db, group)
	if err != nil {
		log.Warning("deleteGroupAndDependencies: Cannot delete group project %s: %s\n", group.Name, err)
		return err
	}

	err = deleteGroup(db, group)
	if err != nil {
		log.Warning("deleteGroupAndDependencies: Cannot delete group %s: %s\n", group.Name, err)
		return err
	}

	return nil
}

// AddGroup creates a new group in database
func AddGroup(db database.QueryExecuter, group *sdk.Group) (int64, bool, error) {
	// check projectKey pattern
	regexp := regexp.MustCompile(sdk.NamePattern)
	if !regexp.MatchString(group.Name) {
		log.Warning("AddGroup: Wrong pattern for group name : %s\n", group.Name)
		return 0, false, sdk.ErrInvalidGroupPattern
	}

	// Check that group does not already exists
	query := `SELECT id FROM "group" WHERE "group".name = $1`
	rows, err := db.Query(query, group.Name)
	if err != nil {
		log.Warning("AddGroup: Cannot check if group %s exist: %s\n", group.Name, err)
		return 0, false, err
	}
	defer rows.Close()

	if rows.Next() {
		log.Warning("AddGroup: Group %s already exists\n", group.Name)

		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			log.Warning("AddGroup: Cannot get the ID of the existing group %s (%s)\n", group.Name, err)
			return 0, false, sdk.ErrGroupExists
		}

		return groupID, false, sdk.ErrGroupExists
	}

	err = InsertGroup(db, group)
	if err != nil {
		log.Warning("AddGroup: Cannot insert group: %s\n", err)
		return 0, false, err
	}
	return group.ID, true, nil
}

// LoadGroup retrieves group informations from database
func LoadGroup(db database.Querier, name string) (*sdk.Group, error) {
	query := `SELECT "group".id FROM "group" WHERE "group".name = $1`
	var groupID int64
	err := db.QueryRow(query, name).Scan(&groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sdk.ErrGroupNotFound
		}
		return nil, err
	}
	return &sdk.Group{
		ID:   groupID,
		Name: name,
	}, nil
}

// LoadGroupByID retrieves group informations from database
func LoadGroupByID(db database.Querier, id int64) (*sdk.Group, error) {
	query := `SELECT "group".id FROM "group" WHERE "group".id = $1`
	var name string
	if err := db.QueryRow(query, name).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil, sdk.ErrGroupNotFound
		}
		return nil, err
	}
	return &sdk.Group{
		ID:   id,
		Name: name,
	}, nil
}

// LoadUserGroup retrieves all group users from database
func LoadUserGroup(db *sql.DB, group *sdk.Group) error {
	query := `SELECT "user".username, "group_user".group_admin FROM "user"
	 		  JOIN group_user ON group_user.user_id = "user".id
	 		  WHERE group_user.group_id = $1 ORDER BY "user".username ASC`

	rows, err := db.Query(query, group.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userName string
		var admin bool
		if err := rows.Scan(&userName, &admin); err != nil {
			return err
		}
		u := sdk.User{Username: userName}
		if admin {
			group.Admins = append(group.Admins, u)
		} else {
			group.Users = append(group.Users, u)
		}
	}
	return nil
}

// LoadGroups load all groups from database
func LoadGroups(db *sql.DB) ([]sdk.Group, error) {
	groups := []sdk.Group{}

	query := `SELECT * FROM "group"`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		g := sdk.NewGroup(name)
		g.ID = id
		groups = append(groups, *g)
	}
	return groups, nil
}

//LoadGroupByUser return group list from database
func LoadGroupByUser(db database.Querier, userID int64) ([]sdk.Group, error) {
	groups := []sdk.Group{}

	query := `
		SELECT "group".id, "group".name
		FROM "group"
		JOIN "group_user" ON "group".id = "group_user".group_id
		WHERE "group_user".user_id = $1
		`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		g := sdk.NewGroup(name)
		g.ID = id
		groups = append(groups, *g)
	}
	return groups, nil
}

//LoadGroupByAdmin return group list from database
func LoadGroupByAdmin(db database.Querier, userID int64) ([]sdk.Group, error) {
	groups := []sdk.Group{}

	query := `
		SELECT "group".id, "group".name
		FROM "group"
		JOIN "group_user" ON "group".id = "group_user".group_id
		WHERE "group_user".user_id = $1
		and "group_user".group_admin = true
		`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		g := sdk.NewGroup(name)
		g.ID = id
		groups = append(groups, *g)
	}
	return groups, nil
}

// LoadPublicGroups returns public groups,
// actually it returns shared.infra group only because public group are not supported
func LoadPublicGroups(db database.Querier) ([]sdk.Group, error) {
	groups := []sdk.Group{}
	//This query should have to be fixed with a new public column
	query := `
		SELECT id, name
		FROM "group"
		WHERE name = $1
		`
	rows, err := db.Query(query, SharedInfraGroup)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		g := sdk.NewGroup(name)
		g.ID = id
		groups = append(groups, *g)
	}
	return groups, nil
}

// CheckUserInGroup verivies that user is in given group
func CheckUserInGroup(db *sql.DB, groupID, userID int64) (bool, error) {
	query := `SELECT COUNT(user_id) FROM group_user WHERE group_id = $1 AND user_id = $2`

	var nb int64
	err := db.QueryRow(query, groupID, userID).Scan(&nb)
	if err != nil {
		return false, err
	}

	if nb == 1 {
		return true, nil
	}

	return false, nil
}

// DeleteUserFromGroup remove user from group
func DeleteUserFromGroup(db *sql.DB, groupID, userID int64) error {

	// Check if there are admin left
	var isAdm bool
	err := db.QueryRow(`SELECT group_admin FROM "group_user" WHERE group_id = $1 AND user_id = $2`, groupID, userID).Scan(&isAdm)
	if err != nil {
		return err
	}

	if isAdm {
		var nbAdm int
		err = db.QueryRow(`SELECT COUNT(id) FROM "group_user" WHERE group_id = $1 AND group_admin = true`, groupID).Scan(&nbAdm)
		if err != nil {
			return err
		}

		if nbAdm <= 1 {
			return sdk.ErrNotEnoughAdmin
		}
	}

	query := `DELETE FROM group_user WHERE group_id=$1 AND user_id=$2`
	_, err = db.Exec(query, groupID, userID)
	return err
}

// InsertUserInGroup insert user in group
func InsertUserInGroup(db database.Executer, groupID, userID int64, admin bool) error {
	query := `INSERT INTO group_user (group_id,user_id,group_admin) VALUES($1,$2,$3)`
	_, err := db.Exec(query, groupID, userID, admin)
	return err
}

// DeleteGroupUserByGroup Delete all user from a group
func DeleteGroupUserByGroup(db database.Executer, group *sdk.Group) error {
	query := `DELETE FROM group_user WHERE group_id=$1`
	_, err := db.Exec(query, group.ID)
	return err
}

// UpdateGroup updates group informations in database
func UpdateGroup(db database.Executer, g *sdk.Group, oldName string) error {
	query := `UPDATE "group" set name=$1 WHERE name=$2`
	_, err := db.Exec(query, g.Name, oldName)

	if err != nil && strings.Contains(err.Error(), "idx_group_name") {
		return sdk.ErrGroupExists
	}

	return err
}

// InsertGroup insert given group into given database
func InsertGroup(db database.QueryExecuter, g *sdk.Group) error {
	query := `INSERT INTO "group" (name) VALUES($1) RETURNING id`
	err := db.QueryRow(query, g.Name).Scan(&g.ID)
	return err
}

// LoadGroupByProject retrieves all groups related to project
func LoadGroupByProject(db database.Querier, project *sdk.Project) error {
	query := `SELECT "group".id,"group".name,project_group.role FROM "group"
	 		  JOIN project_group ON project_group.group_id = "group".id
	 		  WHERE project_group.project_id = $1 ORDER BY "group".name ASC`

	rows, err := db.Query(query, project.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var group sdk.Group
		var perm int
		if err := rows.Scan(&group.ID, &group.Name, &perm); err != nil {
			return err
		}
		project.ProjectGroups = append(project.ProjectGroups, sdk.GroupPermission{
			Group:      group,
			Permission: perm,
		})
	}
	return nil
}

func deleteGroup(db database.Executer, g *sdk.Group) error {
	query := `DELETE FROM "group" WHERE id=$1`
	_, err := db.Exec(query, g.ID)
	return err
}

// SetUserGroupAdmin allows a user to perform operations on given group
func SetUserGroupAdmin(db database.Executer, groupID int64, userID int64) error {
	query := `UPDATE "group_user" SET group_admin = true WHERE group_id = $1 AND user_id = $2`

	res, errE := db.Exec(query, groupID, userID)
	if errE != nil {
		return errE
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return fmt.Errorf("cannot set user %d group admin of %d", userID, groupID)
	}

	return nil
}

// RemoveUserGroupAdmin remove the privilege to perform operations on given group
func RemoveUserGroupAdmin(db *sql.DB, groupID int64, userID int64) error {
	query := `UPDATE "group_user" SET group_admin = false WHERE group_id = $1 AND user_id = $2`
	if _, err := db.Exec(query, groupID, userID); err != nil {
		return err
	}
	return nil
}
