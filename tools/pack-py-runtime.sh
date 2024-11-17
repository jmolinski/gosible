#!/bin/bash
set -e
cd py_runtime
find . -type d -name  "__pycache__" -exec rm -r {} +
rm -f py_runtime.zip
zip -q -r py_runtime.zip ansible/ py_runtime/
cd ..
mv py_runtime/py_runtime.zip bin/remote/py_runtime.zip
