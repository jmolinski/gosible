package gosibleModule

import (
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/file"
	"github.com/scylladb/gosible/utils/osUtils"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"syscall"
)

var modeOperatorRe = regexp.MustCompile(`[+=-]`)
var wrongUserRe = regexp.MustCompile(`[^ugo]`)
var illegalPermRe = regexp.MustCompile(`[^rwxXstugo]`)

var getUmask = osUtils.GetUmask

// https://cs.opensource.google/go/go/+/release-branch.go1.18:src/os/file_posix.go;l=62
// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
func syscallMode(i os.FileMode) (o os.FileMode) {
	o |= i.Perm()
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	// No mapping for Go's ModeTemporary (plan9 only).
	return
}

// symbolicModeToOcta enables symbolic chmod string parsing as stated in the chmod man-page
// This includes things like: "u=rw-x+X,g=r-x+X,o=r-x+X"
func symbolicModeToOctal(stat os.FileInfo, symbolicMode string) (os.FileMode, error) {
	newMode := syscallMode(stat.Mode())
	// Now parse all symbolic modes.
	for _, mode := range strings.Split(symbolicMode, ",") {
		// Per single mode. This always contains a '+', '-' or '='. Split it on that.
		permList := modeOperatorRe.Split(mode, -1)
		if len(permList) == 0 {
			return 0, errors.New("wrong symbolic mode format")
		}
		// And find all the operators.
		opers := modeOperatorRe.FindAllString(mode, -1)
		// The user(s) where it's all about is the first element in the 'permlist' list.
		// Take that and remove it from the list. An empty user or 'a' means 'all'.
		users := permList[0]
		permList = permList[1:]
		useUmask := users == ""
		if users == "" || users == "a" {
			users = "ugo"
		}
		// Check if there are illegal characters in the user list
		// They can end up in 'users' because they are not split
		if wrongUserRe.MatchString(users) {
			return 0, fmt.Errorf("bad symbolic permission for mode: %s", mode)
		}
		// Check if length is the same
		if len(opers) != len(permList) {
			return 0, fmt.Errorf("bad symbolic permission for mode: %s", mode)
		}
		// Now we have two list of equal length, one contains the requested
		// permissions and one with the corresponding operators.
		for i, perms := range permList {
			// Check if there are illegal characters in the permissions
			if illegalPermRe.MatchString(perms) {
				return 0, fmt.Errorf("bad symbolic permission for mode: %s", mode)
			}
			for _, user := range users {
				modeToApply := getOctalModeFromSymbolicPerms(stat, user, perms, useUmask)
				newMode = applyOperationToMode(user, opers[i], modeToApply, newMode)
			}
		}
	}
	return newMode, nil
}

func applyOperationToMode(user rune, op string, modeToApply os.FileMode, currentMode os.FileMode) fs.FileMode {
	var newMode os.FileMode
	switch op {
	case "=":
		var mask os.FileMode
		switch user {
		case 'u':
			mask = syscall.S_IRWXU | syscall.S_ISUID
		case 'g':
			mask = syscall.S_IRWXG | syscall.S_ISGID
		case '0':
			mask = syscall.S_IRWXO | syscall.S_ISVTX
		}
		// mask out u, g, or o permissions from current_mode and apply new permissions
		inverseMask := mask ^ file.PermBits
		newMode = (currentMode & inverseMask) | modeToApply
	case "+":
		newMode = currentMode | modeToApply
	case "-":
		newMode = currentMode - (currentMode & modeToApply)
	}
	return newMode
}

func getOctalModeFromSymbolicPerms(stat os.FileInfo, user rune, perms string, useUmask bool) os.FileMode {
	userPermsToMode := getUserPermsToModesMap(stat, useUmask)
	var mode os.FileMode
	for _, perm := range perms {
		mode |= userPermsToMode[user][perm]
	}
	return mode
}

func getUserPermsToModesMap(stat os.FileInfo, useUmask bool) map[rune]map[rune]os.FileMode {
	prevMode := syscallMode(stat.Mode())
	isDir := stat.IsDir()

	hasXPermissions := prevMode&file.ExecPermBits != 0
	applyXPermission := isDir || hasXPermissions
	// Get the umask, if the 'user' part is empty, the effect is as if (a) were
	// given, but bits that are set in the umask are not affected.
	// We also need the "reversed umask" for masking
	umask := getUmask()
	revUmask := os.FileMode(umask) ^ file.PermBits
	if !useUmask {
		// All bits set to not have any effect.
		revUmask = ^os.FileMode(0)
	}

	var uX, gX, oX os.FileMode
	if applyXPermission {
		uX = syscall.S_IXUSR
		gX = syscall.S_IXGRP
		oX = syscall.S_IXOTH
	}

	return map[rune]map[rune]os.FileMode{
		'u': {
			'r': revUmask & syscall.S_IRUSR,
			'w': revUmask & syscall.S_IWUSR,
			'x': revUmask & syscall.S_IXUSR,
			's': syscall.S_ISUID,
			't': 0,
			'u': prevMode & syscall.S_IRWXU,
			'g': (prevMode & syscall.S_IRWXG) << 3,
			'o': (prevMode & syscall.S_IRWXO) << 6,
			'X': uX,
		},
		'g': {
			'r': revUmask & syscall.S_IRGRP,
			'w': revUmask & syscall.S_IWGRP,
			'x': revUmask & syscall.S_IXGRP,
			's': syscall.S_ISGID,
			't': 0,
			'u': (prevMode & syscall.S_IRWXU) >> 3,
			'g': prevMode & syscall.S_IRWXG,
			'o': (prevMode & syscall.S_IRWXO) << 3,
			'X': gX,
		},
		'o': {
			'r': revUmask & syscall.S_IROTH,
			'w': revUmask & syscall.S_IWOTH,
			'x': revUmask & syscall.S_IXOTH,
			's': 0,
			't': syscall.S_ISVTX,
			'u': (prevMode & syscall.S_IRWXU) >> 6,
			'g': (prevMode & syscall.S_IRWXG) >> 3,
			'o': prevMode & syscall.S_IRWXO,
			'X': oX,
		},
	}
}
