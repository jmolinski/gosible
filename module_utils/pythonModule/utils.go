package pythonModule

import "os"

func SaveRuntimeData(data []byte) error {
	path, err := getRuntimePath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
