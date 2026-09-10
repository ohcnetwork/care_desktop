package mdns

import (
	"fmt"
	"regexp"
	"strings"
)

var labelRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// ValidateLabel checks a user-supplied name. The installer calls it as the
// user types.
func ValidateLabel(name string) error {
	label := strings.ToLower(Label(name))
	switch {
	case label == "":
		return fmt.Errorf("enter a name, for example care")
	case len(label) > 63:
		return fmt.Errorf("name is too long (63 characters at most)")
	case !labelRe.MatchString(label):
		return fmt.Errorf("use lowercase letters, numbers and hyphens only, for example care-test")
	}
	return nil
}
