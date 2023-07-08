package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	dboperatorv1alpha1 "github.com/obeleh/db-operator/api/v1alpha1"
	funk "github.com/thoas/go-funk"
)

type privsGetter func(conn *sql.DB, user string, scopedName string) ([]string, error)
type privsReconcilerConstructor func(privs dboperatorv1alpha1.DbPriv, conn *sql.DB, userName string, scopedName string, normalizedPrivSet []string, serverVersion *PostgresVersion) (*PrivsReconciler, error)

var PRIVS_RECONCILER_CONSTRUCTORS = map[string]privsReconcilerConstructor{
	"table":        NewTablePrivsReconciler,
	"defaultTable": NewDefaultTablePrivsReconciler,
	"database":     NewDatabasePrivsReconciler,
	"schema":       NewSchemaPrivsReconciler,
}

func diffPrivSet(curPrivs []string, privs []string) ([]string, []string, []string) {
	haveCurrently := funk.Join(curPrivs, privs, funk.InnerJoin).([]string)
	otherCurrent, desired := funk.Difference(curPrivs, privs)
	return haveCurrently, otherCurrent.([]string), desired.([]string)
}

type PrivsReconciler struct {
	dboperatorv1alpha1.DbPriv
	DesiredPrivSet          []string
	FoundPrivSet            []string
	UserName                string
	scopedName              string
	IsDefaultPrivReconciler bool
	DefaultPrivObjectType   string

	conn *sql.DB

	grantFun    privsAdjuster
	revokeFun   privsAdjuster
	privsGetFun privsGetter
}

func (r *PrivsReconciler) GetConn() *sql.DB {
	return r.conn
}

func (r *PrivsReconciler) ReconcilePrivs() (bool, error) {
	curPrivs, err := r.privsGetFun(r.conn, r.UserName, r.scopedName)
	if err != nil {
		return false, err
	}
	r.FoundPrivSet = curPrivs
	_, toRevoke, toGrant := diffPrivSet(curPrivs, r.DesiredPrivSet)

	changed := false
	if len(toRevoke) > 0 {
		err = r.revokeFun(r.conn, r.UserName, r.scopedName, toRevoke)
		if err != nil {
			return changed, err
		}
		changed = true
	}

	if len(toGrant) > 0 {
		err = r.grantFun(r.conn, r.UserName, r.scopedName, toGrant)
		if err != nil {
			return changed, err
		}
		changed = true
	}

	return changed, nil
}

func (r *PrivsReconciler) RevokeAllPrivs() error {
	return r.revokeFun(r.conn, r.UserName, r.scopedName, r.DesiredPrivSet)
}

func normalizeDbPriv(dbPriv dboperatorv1alpha1.DbPriv, dbName string) (string, string, []string, error) {
	privType := ""
	scopedName := ""
	var privSet []string
	if dbPriv.PrivType != "" {
		privType = dbPriv.PrivType
		scopedName = dbPriv.Scope
		if dbPriv.DefaultPrivs != "" {
			return "", "", nil, fmt.Errorf("DefaultPrivs no longer supported with PrivType set")
		}
		privSet = toPrivSet(dbPriv.Privs)
		if privType == "defaultTable" {
			if dbPriv.Grantor != nil {
				scopedName = *dbPriv.Grantor
			} else {
				scopedName = ""
			}
		} else if privType == "schema" {
			var err error
			scopedName, err = GetScopeAfterDb(dbPriv.Scope)
			if err != nil {
				return "", "", nil, err
			}
		}
	} else {
		if dbPriv.Privs != "" {
			if strings.Contains(dbPriv.Privs, ":") {
				privType = "table"
				elements := strings.Split(dbPriv.Privs, ":")
				scopedName = elements[0]
				privileges := elements[1]
				privSet = toPrivSet(privileges)
			} else if strings.Contains(dbPriv.Scope, ".") {
				privType = "schema"
				//scopedName = dbPriv.Scope
				var err error
				scopedName, err = GetScopeAfterDb(dbPriv.Scope)
				if err != nil {
					return "", "", nil, err
				}
				privSet = toPrivSet(dbPriv.Privs)
			} else {
				privType = "database"
				scopedName = dbName
				privSet = toPrivSet(dbPriv.Privs)
			}
		} else if dbPriv.DefaultPrivs != "" {
			println("defaultPrivs are deprecated, use privType=\"defaultTable\" property instead")
			scopeAfterDb, err := GetScopeAfterDb(dbPriv.Scope)
			if err != nil {
				return "", "", nil, err
			}
			if scopeAfterDb == "TABLES" {
				if dbPriv.Grantor != nil {
					scopedName = *dbPriv.Grantor
				}
				privSet = toPrivSet(dbPriv.DefaultPrivs)
				privType = "defaultTable"
			} else {
				return "", "", nil, fmt.Errorf(fmt.Sprintf("Not implemented to update default privileges on %s", scopeAfterDb))
			}
		} else {
			return "", "", nil, fmt.Errorf("no privs or default privs specified")
		}
	}
	return privType, scopedName, privSet, nil
}

func GetPrivsReconciler(userName string, dbPriv dboperatorv1alpha1.DbPriv, serverVersion *PostgresVersion, connectionGetter ConnectionGetter) (*PrivsReconciler, error) {
	dbName := GetDbNameFromScopeName(dbPriv.Scope)
	conn, err := connectionGetter(dbPriv.Grantor, &dbName)
	if err != nil {
		return nil, err
	}

	if dbPriv.Privs != "" && dbPriv.DefaultPrivs != "" {
		return nil, fmt.Errorf("both privs and default privs specified")
	}

	if strings.Contains(dbPriv.Privs, "/") {
		return nil, fmt.Errorf("privs cannot contain '/' this is deprecated")
	}

	privType, scopedName, privSet, err := normalizeDbPriv(dbPriv, dbName)
	if err != nil {
		return nil, err
	}

	strippedPrivType := strings.ToLower(strings.TrimPrefix(privType, "default"))
	if !funk.Subset(privSet, serverVersion.GetValidPrivs(strippedPrivType)) {
		invalidPrivs := strings.Join(funk.Subtract(privSet, serverVersion.GetValidPrivs(strippedPrivType)).([]string), " ")
		return nil, fmt.Errorf("invalid privs specified for %s: %s", strippedPrivType, invalidPrivs)
	}
	privSet = NormalizePrivileges(privSet, privType, serverVersion)

	constructor, ok := PRIVS_RECONCILER_CONSTRUCTORS[privType]
	if !ok {
		return nil, fmt.Errorf("invalid privType: %s", privType)
	}

	return constructor(dbPriv, conn, userName, scopedName, privSet, serverVersion)
}
