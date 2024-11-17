package constants

var MagicVariableMapping = map[string][]string{
	// base
	"connection":         {"ansible_connection"},
	"module_compression": {"ansible_module_compression"},
	"shell":              {"ansible_shell_type"},
	"executable":         {"ansible_shell_executable"},

	// connection common
	"remote_addr":      {"ansible_ssh_host", "ansible_host"},
	"remote_user":      {"ansible_ssh_user", "ansible_user"},
	"password":         {"ansible_ssh_pass", "ansible_password"},
	"port":             {"ansible_ssh_port", "ansible_port"},
	"pipelining":       {"ansible_ssh_pipelining", "ansible_pipelining"},
	"timeout":          {"ansible_ssh_timeout", "ansible_timeout"},
	"private_key_file": {"ansible_ssh_private_key_file", "ansible_private_key_file"},

	// networking modules
	"network_os":      {"ansible_network_os"},
	"connection_user": {"ansible_connection_user"},

	// ssh
	"ssh_executable":      {"ansible_ssh_executable"},
	"ssh_common_args":     {"ansible_ssh_common_args"},
	"sftp_extra_args":     {"ansible_sftp_extra_args"},
	"scp_extra_args":      {"ansible_scp_extra_args"},
	"ssh_extra_args":      {"ansible_ssh_extra_args"},
	"ssh_transfer_method": {"ansible_ssh_transfer_method"},

	// docker
	"docker_extra_args": {"ansible_docker_extra_args"},

	// become
	"become":        {"ansible_become"},
	"become_method": {"ansible_become_method"},
	"become_user":   {"ansible_become_user"},
	"become_pass":   {"ansible_become_password", "ansible_become_pass"},
	"become_exe":    {"ansible_become_exe"},
	"become_flags":  {"ansible_become_flags"},
}

func MagicVariableMappedNames(key string) []string {
	v, _ := MagicVariableMapping[key]
	// If the key is not found, returns a nil slice - it's safe to iterate.
	return v
}
