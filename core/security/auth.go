package security

import (
	"context"
	"sync"
)

// Permission 权限定义

type Permission string

// 常见权限
const (
	PermissionRead   Permission = "read"
	PermissionWrite  Permission = "write"
	PermissionDelete Permission = "delete"
	PermissionAdmin  Permission = "admin"
)

// Role 角色定义

type Role struct {
	ID          string
	Name        string
	Description string
	Permissions []Permission
}

// User  用户信息

type User struct {
	ID       string
	Username string
	Roles    []string // 角色ID列表
}

// AuthManager 应用级权限管理器

type AuthManager struct {
	mutex         sync.RWMutex
	users         map[string]*User
	roles         map[string]*Role
	enabled       bool
	defaultPolicy string // allow, deny
}

// NewAuthManager 创建权限管理器实例
func NewAuthManager(defaultPolicy string) *AuthManager {
	am := &AuthManager{
		users:         make(map[string]*User),
		roles:         make(map[string]*Role),
		enabled:       true,
		defaultPolicy: defaultPolicy,
	}

	// 创建默认角色
	am.createDefaultRoles()

	return am
}

// 创建默认角色
func (am *AuthManager) createDefaultRoles() {
	// 访客角色
	am.AddRole(&Role{
		ID:          "guest",
		Name:        "Guest",
		Description: "Default role for unauthenticated users",
		Permissions: []Permission{PermissionRead},
	})

	// 用户角色
	am.AddRole(&Role{
		ID:          "user",
		Name:        "User",
		Description: "Default role for authenticated users",
		Permissions: []Permission{PermissionRead, PermissionWrite},
	})

	// 管理员角色
	am.AddRole(&Role{
		ID:          "admin",
		Name:        "Administrator",
		Description: "Administrator role with full access",
		Permissions: []Permission{PermissionRead, PermissionWrite, PermissionDelete, PermissionAdmin},
	})
}

// AddUser 添加用户
func (am *AuthManager) AddUser(user *User) {
	am.mutex.Lock()
	am.users[user.ID] = user
	am.mutex.Unlock()
}

// RemoveUser 移除用户
func (am *AuthManager) RemoveUser(userID string) bool {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if _, exists := am.users[userID]; exists {
		delete(am.users, userID)
		return true
	}

	return false
}

// AddRole 添加角色
func (am *AuthManager) AddRole(role *Role) {
	am.mutex.Lock()
	am.roles[role.ID] = role
	am.mutex.Unlock()
}

// RemoveRole 移除角色
func (am *AuthManager) RemoveRole(roleID string) bool {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if _, exists := am.roles[roleID]; exists {
		delete(am.roles, roleID)
		return true
	}

	return false
}

// CheckPermission 检查权限
func (am *AuthManager) CheckPermission(ctx context.Context, userID string, permission Permission) bool {
	if !am.enabled {
		// 如果未启用权限检查，默认允许
		return true
	}

	am.mutex.RLock()
	defer am.mutex.RUnlock()

	// 检查用户是否存在
	user, exists := am.users[userID]
	if !exists {
		// 用户不存在，应用默认策略
		return am.defaultPolicy == "allow"
	}

	// 检查用户角色的权限
	for _, roleID := range user.Roles {
		role, roleExists := am.roles[roleID]
		if !roleExists {
			continue
		}

		// 检查角色是否有该权限
		for _, p := range role.Permissions {
			if p == permission {
				return true
			}
		}

		// 管理员角色拥有所有权限
		if role.ID == "admin" {
			return true
		}
	}

	// 没有找到匹配的权限，应用默认策略
	return am.defaultPolicy == "allow"
}

// GetUser 获取用户信息
func (am *AuthManager) GetUser(userID string) (*User, bool) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	user, exists := am.users[userID]
	return user, exists
}

// GetRole 获取角色信息
func (am *AuthManager) GetRole(roleID string) (*Role, bool) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	role, exists := am.roles[roleID]
	return role, exists
}

// Enable 启用权限检查
func (am *AuthManager) Enable() {
	am.mutex.Lock()
	am.enabled = true
	am.mutex.Unlock()
}

// Disable 禁用权限检查
func (am *AuthManager) Disable() {
	am.mutex.Lock()
	am.enabled = false
	am.mutex.Unlock()
}
