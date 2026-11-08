package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// permissionBits maps Discord permission flag names to bit positions.
var permissionBits = map[string]uint{
	"CREATE_INSTANT_INVITE":               0,
	"KICK_MEMBERS":                        1,
	"BAN_MEMBERS":                         2,
	"ADMINISTRATOR":                       3,
	"MANAGE_CHANNELS":                     4,
	"MANAGE_GUILD":                        5,
	"ADD_REACTIONS":                       6,
	"VIEW_AUDIT_LOG":                      7,
	"PRIORITY_SPEAKER":                    8,
	"STREAM":                              9,
	"VIEW_CHANNEL":                        10,
	"SEND_MESSAGES":                       11,
	"SEND_TTS_MESSAGES":                   12,
	"MANAGE_MESSAGES":                     13,
	"EMBED_LINKS":                         14,
	"ATTACH_FILES":                        15,
	"READ_MESSAGE_HISTORY":                16,
	"MENTION_EVERYONE":                    17,
	"USE_EXTERNAL_EMOJIS":                 18,
	"VIEW_GUILD_INSIGHTS":                 19,
	"CONNECT":                             20,
	"SPEAK":                               21,
	"MUTE_MEMBERS":                        22,
	"DEAFEN_MEMBERS":                      23,
	"MOVE_MEMBERS":                        24,
	"USE_VAD":                             25,
	"CHANGE_NICKNAME":                     26,
	"MANAGE_NICKNAMES":                    27,
	"MANAGE_ROLES":                        28,
	"MANAGE_WEBHOOKS":                     29,
	"MANAGE_GUILD_EXPRESSIONS":            30,
	"USE_APPLICATION_COMMANDS":            31,
	"REQUEST_TO_SPEAK":                    32,
	"MANAGE_EVENTS":                       33,
	"MANAGE_THREADS":                      34,
	"CREATE_PUBLIC_THREADS":               35,
	"CREATE_PRIVATE_THREADS":              36,
	"USE_EXTERNAL_STICKERS":               37,
	"SEND_MESSAGES_IN_THREADS":            38,
	"USE_EMBEDDED_ACTIVITIES":             39,
	"MODERATE_MEMBERS":                    40,
	"VIEW_CREATOR_MONETIZATION_ANALYTICS": 41,
	"USE_SOUNDBOARD":                      42,
	"CREATE_GUILD_EXPRESSIONS":            43,
	"CREATE_EVENTS":                       44,
	"USE_EXTERNAL_SOUNDS":                 45,
	"SEND_VOICE_MESSAGES":                 46,
	"USE_CLYDE_AI":                        47, // retired, still set on older @everyone roles
	"SET_VOICE_CHANNEL_STATUS":            48,
	"SEND_POLLS":                          49,
	"USE_EXTERNAL_APPS":                   50,
	"PIN_MESSAGES":                        51,
	"BYPASS_SLOWMODE":                     52,
}

// PermissionsToBits converts flag names to a decimal bitfield string.
func PermissionsToBits(names []string) (string, error) {
	var v uint64
	for _, n := range names {
		key := strings.ToUpper(strings.TrimSpace(n))
		bit, ok := permissionBits[key]
		if !ok {
			return "", fmt.Errorf("unknown permission %q", n)
		}
		v |= 1 << bit
	}
	return strconv.FormatUint(v, 10), nil
}

// BitsToPermissions converts a decimal bitfield string to flag names.
func BitsToPermissions(bits string) ([]string, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(bits), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bitfield %q: %w", bits, err)
	}
	byBit := make(map[uint]string, len(permissionBits))
	for n, b := range permissionBits {
		byBit[b] = n
	}
	var out []string
	for bit := range uint(64) {
		if v&(1<<bit) == 0 {
			continue
		}
		if n, ok := byBit[bit]; ok {
			out = append(out, n)
		} else {
			out = append(out, fmt.Sprintf("UNKNOWN_BIT_%d", bit))
		}
	}
	return out, nil
}

func permissionsTool() (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("permissions",
		mcp.WithDescription("Convert between Discord permission names and bitfield strings (local, no API call). Pass names to get a bitfield, or bits to get names. With neither, lists all known names."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithArray("names", mcp.Description("Permission names, e.g. [\"VIEW_CHANNEL\", \"SEND_MESSAGES\"]"), mcp.WithStringItems()),
		mcp.WithString("bits", mcp.Description("Decimal permission bitfield, e.g. \"3072\"")),
	)
	handler := func(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		names := r.GetStringSlice("names", nil)
		bits := r.GetString("bits", "")
		switch {
		case len(names) > 0:
			v, err := PermissionsToBits(names)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(v), nil
		case bits != "":
			ns, err := BitsToPermissions(bits)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(strings.Join(ns, ", ")), nil
		default:
			all := make([]string, 0, len(permissionBits))
			for n := range permissionBits {
				all = append(all, n)
			}
			sort.Slice(all, func(i, j int) bool { return permissionBits[all[i]] < permissionBits[all[j]] })
			return mcp.NewToolResultText(strings.Join(all, ", ")), nil
		}
	}
	return tool, handler
}
