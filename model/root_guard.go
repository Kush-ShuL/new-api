package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrUnauthorizedRootUser = errors.New("root account identity is invalid")

type UnauthorizedRootUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Status   int    `json:"status"`
}

// QuarantineUnauthorizedRootUsers also returns previously quarantined accounts
// so the canonical root can review them without automatically deleting evidence.
func QuarantineUnauthorizedRootUsers() ([]UnauthorizedRootUser, error) {
	var users []UnauthorizedRootUser
	if err := DB.Model(&User{}).Where("id <> ? AND role >= ?", 1, common.RoleRootUser).
		Order("id").Select("id", "username", "status").Find(&users).Error; err != nil {
		return nil, err
	}
	for i := range users {
		if err := quarantineUnauthorizedRootUser(users[i].ID); err != nil {
			return nil, err
		}
		users[i].Status = common.UserStatusDisabled
	}
	return users, nil
}

func quarantineUnauthorizedRootUser(userID int) error {
	if userID <= 1 {
		return ErrUnauthorizedRootUser
	}
	var nextVersion int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.Role < common.RoleRootUser || user.Status == common.UserStatusDisabled {
			return nil
		}
		var err error
		nextVersion, err = IncrementUserAuthVersionWithTx(tx, userID)
		if err != nil {
			return err
		}
		if _, err = DeleteUserAccessTokensWithTx(tx, userID); err != nil {
			return err
		}
		return tx.Model(&user).Updates(map[string]any{
			"status": common.UserStatusDisabled, "access_token": nil,
		}).Error
	})
	if err != nil || nextVersion == 0 {
		return err
	}
	if err := publishCommittedUserAuthVersion(userID, nextVersion); err != nil {
		return err
	}
	if err := invalidateUserCache(userID); err != nil {
		return err
	}
	if _, err := RevokeAllUserSessions(userID, "unauthorized_root_account"); err != nil {
		return err
	}
	common.SysLog(common.LogText("Unauthorized root account %d quarantined; authentication revoked", userID))
	return nil
}
