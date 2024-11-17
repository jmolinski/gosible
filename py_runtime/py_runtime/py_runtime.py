import sys
import json
import importlib.util
from ansible.module_utils.common.text.converters import jsonify


INIT_CODE = """
from py_runtime.init_module import init

init(_INIT_OPTIONS)
"""
INIT_CODE_COMPILED = compile(INIT_CODE, '#INIT_CODE', 'exec')

cached_modules = {}


def execute(module_name, module_args):
    if module_name not in cached_modules:
        spec = importlib.util.find_spec(module_name)
        module_code = spec.loader.get_code(module_name)
        cached_modules[module_name] = module_code

    result = None

    def callback(new_result):
        nonlocal result
        result = new_result

    init_options = {
        'args': module_args,
        'callback': callback
    }
    globals = {
        '__name__': '__main__',
        '__package__': module_name.rpartition('.')[0],
        '_INIT_OPTIONS': init_options
    }
    exec(INIT_CODE_COMPILED, globals, globals)
    del globals['_INIT_OPTIONS']
    try:
        exec(cached_modules[module_name], globals, globals)
    except SystemExit:
        pass

    return result


def handle_hello(_):
    return {}


def handle_execute(data):
    try:
        assert data['Args'] is not None
        result = execute(data['ModuleName'], data['Args'])
        return {'Result': result}
    except Exception as e:
        return {'Exception': str(e)}


handlers = {
    'hello': handle_hello,
    'execute': handle_execute
}

while True:
    raw_hdr = sys.stdin.readline()
    if raw_hdr == '':
        break
    hdr = json.loads(raw_hdr)
    data = json.loads(sys.stdin.readline())

    rsp_hdr = {"Tag": hdr["Tag"]}
    rsp_data = handlers[hdr["Cmd"]](data)
    print(json.dumps(rsp_hdr))
    print(jsonify(rsp_data), flush=True)



