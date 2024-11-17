package pythonModule

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
)

func getPrimaryPythonInterpreter() string {
	// TODO: possibly use python interpreter from meta vars
	return "/usr/bin/python3"
}

type PythonExecutor struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	invalid bool
	nextTag uint64
}

func newExecutor(runtimeZipPath string) (*PythonExecutor, error) {
	cmd := exec.Command(getPrimaryPythonInterpreter(), "-m", "py_runtime.py_runtime")
	cmd.Env = os.Environ()
	if pythonPath, ok := os.LookupEnv("PYTHONPATH"); ok {
		cmd.Env = append(cmd.Env, "PYTHONPATH="+runtimeZipPath+":"+pythonPath)
	} else {
		cmd.Env = append(cmd.Env, "PYTHONPATH="+runtimeZipPath)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdout)

	executor := &PythonExecutor{
		cmd:     cmd,
		stdin:   stdin,
		scanner: scanner,
		invalid: false,
		nextTag: 1,
	}

	req := helloRequest{}
	rsp := helloResponse{}
	if err = executor.executeCommand(cmdHello, req, &rsp); err != nil {
		return nil, err
	}

	return executor, nil
}

func (e *PythonExecutor) invalidate() {
	e.invalid = true
	_ = e.cmd.Process.Kill()
}

func (e *PythonExecutor) Close() error {
	e.invalidate()
	return nil
}

func (e *PythonExecutor) writeLine(data any) error {
	inJson, err := json.Marshal(data)
	if err != nil {
		return err
	}
	inJson = append(inJson, '\n')
	write, err := e.stdin.Write(inJson)
	if err != nil {
		e.invalidate()
		return err
	}
	if write != len(inJson) {
		e.invalidate()
		return errors.New("incorrect number of bytes written while writing command input")
	}
	return nil
}

func (e *PythonExecutor) readLine(data any) error {
	if !e.scanner.Scan() {
		e.invalidate()
		err := e.scanner.Err()
		if err == nil {
			err = errors.New("reached unexpected end of file when reading command output")
		}
		return err
	}
	outJson := e.scanner.Bytes()
	if err := json.Unmarshal(outJson, data); err != nil {
		e.invalidate()
		return err
	}
	return nil
}

func (e *PythonExecutor) executeCommand(cmd string, req any, rsp any) error {
	if e.invalid {
		return errors.New("cannot use an invalidated executor")
	}

	reqHdr := requestHeader{
		Cmd: cmd,
		Tag: e.nextTag,
	}
	e.nextTag += 1
	if err := e.writeLine(reqHdr); err != nil {
		return err
	}
	if err := e.writeLine(req); err != nil {
		e.invalidate() // at this point any error needs to invalidate the executor as we have already written the hdr
		return err
	}

	rspHdr := responseHeader{}
	if err := e.readLine(&rspHdr); err != nil {
		return err
	}
	if rspHdr.Tag != reqHdr.Tag {
		e.invalidate()
		return errors.New("response header tag doesn't match")
	}
	return e.readLine(&rsp)
}
