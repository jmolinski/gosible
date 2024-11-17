args = None
callback = None


def init(opts):
    global args
    global callback
    args = opts['args']
    callback = opts['callback']


def set_result(result):
    if callback:
        callback(result)
