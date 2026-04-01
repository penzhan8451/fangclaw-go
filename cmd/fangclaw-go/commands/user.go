package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/penzhan8451/fangclaw-go/internal/auth"
	"github.com/spf13/cobra"
)

func NewUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
		Long:  "Manage users for multi-tenant mode (server mode only)",
	}

	cmd.AddCommand(newUserCreateCmd())
	cmd.AddCommand(newUserListCmd())
	cmd.AddCommand(newUserDeleteCmd())
	cmd.AddCommand(newUserUpdateCmd())
	cmd.AddCommand(newUserShowCmd())

	return cmd
}

func newUserCreateCmd() *cobra.Command {
	var (
		username string
		email    string
		password string
		role     string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Run: func(cmd *cobra.Command, args []string) {
			if username == "" {
				fmt.Fprintln(os.Stderr, "Error: username is required")
				os.Exit(1)
			}
			if password == "" {
				fmt.Fprintln(os.Stderr, "Error: password is required")
				os.Exit(1)
			}

			authDBPath, err := auth.GetDefaultAuthDBPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			authManager, err := auth.NewAuthManager(authDBPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating auth manager: %v\n", err)
				os.Exit(1)
			}
			defer authManager.Close()

			userRole := auth.RoleUser
			switch role {
			case "owner":
				userRole = auth.RoleOwner
			case "admin":
				userRole = auth.RoleAdmin
			case "guest":
				userRole = auth.RoleGuest
			}

			user, err := authManager.CreateUser(username, email, password, userRole)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating user: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("User created successfully:\n")
			fmt.Printf("  ID:       %s\n", user.ID)
			fmt.Printf("  Username: %s\n", user.Username)
			fmt.Printf("  Email:    %s\n", user.Email)
			fmt.Printf("  Role:     %s\n", user.Role)
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "Username (required)")
	cmd.Flags().StringVarP(&email, "email", "e", "", "Email address")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password (required)")
	cmd.Flags().StringVarP(&role, "role", "r", "user", "Role (owner, admin, user, guest)")

	cmd.MarkFlagRequired("username")
	cmd.MarkFlagRequired("password")

	return cmd
}

func newUserListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		Run: func(cmd *cobra.Command, args []string) {
			authDBPath, err := auth.GetDefaultAuthDBPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			authManager, err := auth.NewAuthManager(authDBPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating auth manager: %v\n", err)
				os.Exit(1)
			}
			defer authManager.Close()

			users, err := authManager.ListUsers()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing users: %v\n", err)
				os.Exit(1)
			}

			if len(users) == 0 {
				fmt.Println("No users found.")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tUSERNAME\tEMAIL\tROLE\tVIP\tDISABLED\tCREATED")
			for _, u := range users {
				vip := "no"
				if u.IsVIP {
					vip = "yes"
				}
				disabled := "no"
				if u.Disabled {
					disabled = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					u.ID[:8],
					u.Username,
					u.Email,
					u.Role,
					vip,
					disabled,
					u.CreatedAt.Format("2006-01-02"),
				)
			}
			w.Flush()
		},
	}
}

func newUserDeleteCmd() *cobra.Command {
	var userID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user",
		Run: func(cmd *cobra.Command, args []string) {
			if userID == "" {
				fmt.Fprintln(os.Stderr, "Error: user-id is required")
				os.Exit(1)
			}

			authDBPath, err := auth.GetDefaultAuthDBPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			authManager, err := auth.NewAuthManager(authDBPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating auth manager: %v\n", err)
				os.Exit(1)
			}
			defer authManager.Close()

			if err := authManager.DeleteUser(userID); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting user: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("User %s deleted successfully.\n", userID)
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to delete")
	cmd.MarkFlagRequired("user-id")

	return cmd
}

func newUserUpdateCmd() *cobra.Command {
	var (
		userID   string
		email    string
		password string
		role     string
		disable  bool
		enable   bool
		vip      bool
		unvip    bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a user",
		Run: func(cmd *cobra.Command, args []string) {
			if userID == "" {
				fmt.Fprintln(os.Stderr, "Error: user-id is required")
				os.Exit(1)
			}

			authDBPath, err := auth.GetDefaultAuthDBPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			authManager, err := auth.NewAuthManager(authDBPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating auth manager: %v\n", err)
				os.Exit(1)
			}
			defer authManager.Close()

			updates := make(map[string]interface{})

			if email != "" {
				updates["email"] = email
			}
			if password != "" {
				updates["password"] = password
			}
			if role != "" {
				updates["role"] = role
			}
			if disable {
				updates["disabled"] = true
			}
			if enable {
				updates["disabled"] = false
			}
			if vip {
				updates["is_vip"] = true
			}
			if unvip {
				updates["is_vip"] = false
			}

			if len(updates) == 0 {
				fmt.Fprintln(os.Stderr, "Error: no updates specified")
				os.Exit(1)
			}

			if err := authManager.UpdateUser(userID, updates); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating user: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("User %s updated successfully.\n", userID)
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to update")
	cmd.Flags().StringVar(&email, "email", "", "New email address")
	cmd.Flags().StringVar(&password, "password", "", "New password")
	cmd.Flags().StringVar(&role, "role", "", "New role (owner, admin, user, guest)")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the user")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the user")
	cmd.Flags().BoolVar(&vip, "vip", false, "Set user as VIP")
	cmd.Flags().BoolVar(&unvip, "unvip", false, "Remove VIP status")

	cmd.MarkFlagRequired("user-id")

	return cmd
}

func newUserShowCmd() *cobra.Command {
	var userID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show user details",
		Run: func(cmd *cobra.Command, args []string) {
			if userID == "" {
				fmt.Fprintln(os.Stderr, "Error: user-id is required")
				os.Exit(1)
			}

			authDBPath, err := auth.GetDefaultAuthDBPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			authManager, err := auth.NewAuthManager(authDBPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating auth manager: %v\n", err)
				os.Exit(1)
			}
			defer authManager.Close()

			user, err := authManager.GetUserByID(userID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting user: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("User Details:\n")
			fmt.Printf("  ID:             %s\n", user.ID)
			fmt.Printf("  Username:       %s\n", user.Username)
			fmt.Printf("  Email:          %s\n", user.Email)
			fmt.Printf("  Role:           %s\n", user.Role)
			fmt.Printf("  VIP:            %v\n", user.IsVIP)
			fmt.Printf("  Disabled:       %v\n", user.Disabled)
			fmt.Printf("  Created:        %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
			if user.LastLogin != nil {
				fmt.Printf("  Last Login:     %s\n", user.LastLogin.Format("2006-01-02 15:04:05"))
			}
			if len(user.APIKeys) > 0 {
				fmt.Printf("  API Keys:       %d\n", len(user.APIKeys))
			}
			if len(user.ChannelBindings) > 0 {
				fmt.Printf("  Channel Bindings:\n")
				for k, v := range user.ChannelBindings {
					fmt.Printf("    %s: %s\n", k, v)
				}
			}
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to show")
	cmd.MarkFlagRequired("user-id")

	return cmd
}
