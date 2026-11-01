package handler

import (
	"encoding/json"
	"image/png"
	"os"
	"testing"
	"unicode/utf8"

	"github.com/dfgh012316/popofinder/internal/linebot/template"
	"github.com/stretchr/testify/require"
)

const richMenuDir = "../../../assets/richmenu/"

// The rich menu is applied with scripts/richmenu/apply.sh (OD-16). Every button must
// send a command this handler answers; any other text would run as a search.
func TestRichMenuSendsCommands(t *testing.T) {
	data, err := os.ReadFile(richMenuDir + "richmenu.json")
	require.NoError(t, err)

	type size struct{ Width, Height int }
	var menu struct {
		Size        size
		ChatBarText string
		Areas       []struct {
			Bounds struct{ X, Y, Width, Height int }
			Action struct{ Type, Text string }
		}
	}
	require.NoError(t, json.Unmarshal(data, &menu))
	require.LessOrEqual(t, utf8.RuneCountInString(menu.ChatBarText), 14, "LINE chat bar text limit")

	commands := map[string]bool{helpCommand: true, template.HospitalListCommand: true, template.ReportCommand: true}
	for _, a := range menu.Areas {
		require.Equal(t, "message", a.Action.Type)
		require.True(t, commands[a.Action.Text], "%q is not a command", a.Action.Text)
		b := a.Bounds
		require.True(t, b.X >= 0 && b.Y >= 0 && b.X+b.Width <= menu.Size.Width && b.Y+b.Height <= menu.Size.Height, "area %+v outside the menu", b)
	}

	f, err := os.Open(richMenuDir + "richmenu.png")
	require.NoError(t, err)
	defer f.Close()
	img, err := png.DecodeConfig(f)
	require.NoError(t, err)
	require.Equal(t, size{menu.Size.Width, menu.Size.Height}, size{img.Width, img.Height}, "image size must equal the menu size")
	info, err := f.Stat()
	require.NoError(t, err)
	require.LessOrEqual(t, info.Size(), int64(1<<20), "LINE rich menu image limit")
}
