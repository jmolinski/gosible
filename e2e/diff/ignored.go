package diff

import "regexp"

var ignoredFiles = []*regexp.Regexp{
	regexp.MustCompile(`^/var/log/lastlog$`),
	regexp.MustCompile(`^/var/log/wtmp$`),
	regexp.MustCompile(`^/run/utmp$`),
	regexp.MustCompile(`^/run/sudo`),                                  // Become sudo plugin
	regexp.MustCompile(`^/tmp/tmp\.`),                                 // Gosible binary
	regexp.MustCompile(`^/home/[^/]+/\.cache/gosible_client`),         // Gosible cacheable binary
	regexp.MustCompile(`^/root/\.cache/gosible_client`),               // Gosible cacheable binary
	regexp.MustCompile(`^/usr/lib/python3/dist-packages/__pycache__`), // Pycache
	regexp.MustCompile(`\.ansible`),                                   // Ansible stuff
	regexp.MustCompile(`^/tmp/ansible-moduletmp-`),                    // TODO Gosible doesn't delete Ansible's temp files when using python executor
	regexp.MustCompile(`^/var/log/dpkg.log$`),                         // Apt garbage
	regexp.MustCompile(`^/var/log/apt/term.log$`),                     // Apt garbage
	regexp.MustCompile(`^/var/log/apt/history.log$`),                  // Apt garbage
	regexp.MustCompile(`^/var/cache/ldconfig/aux-cache$`),             // Apt garbage
	regexp.MustCompile(`^/var/log/alternatives.log$`),                 // Apt garbage
	regexp.MustCompile(`\.sudo_as_admin_successful`),                  // Sudo garbage
}

func rejectIgnoredFiles(files []FileDiffDescription) []FileDiffDescription {
	var result []FileDiffDescription
	for _, file := range files {
		if !isIgnoredFile(file.FilePath) {
			result = append(result, file)
		}
	}
	return result
}

func isIgnoredFile(file string) bool {
	for _, ignoredFile := range ignoredFiles {
		if ignoredFile.MatchString(file) {
			return true
		}
	}
	return false
}
