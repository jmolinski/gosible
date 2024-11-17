package file

import "os"

var fileAttributes = map[rune]string{
	'A': "noatime",
	'a': "append",
	'c': "compressed",
	'C': "nocow",
	'd': "nodump",
	'D': "dirsync",
	'e': "extents",
	'E': "encrypted",
	'h': "blocksize",
	'i': "immutable",
	'I': "indexed",
	'j': "journalled",
	'N': "inline",
	's': "zero",
	'S': "synchronous",
	't': "notail",
	'T': "blockroot",
	'u': "undelete",
	'X': "compressedraw",
	'Z': "compresseddirty",
}

func FormatAttributes(attributes string) []string {
	attrList := make([]string, 0, len(attributes))
	for _, attr := range attributes {
		if v, ok := fileAttributes[attr]; ok {
			attrList = append(attrList, v)
		}
	}
	return attrList
}

const DefaultPerm os.FileMode = 0666
const ExecPermBits os.FileMode = 0111
const PermBits os.FileMode = 0777
