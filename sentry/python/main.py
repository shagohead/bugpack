import os
import sys

import custom_exc
import sentry_sdk


def main():
    sentry_sdk.init(dsn=os.environ["DSN"], send_default_pii=True)
    if len(sys.argv) < 2:
        print("missing mandatory argument")
        sys.exit(1)
    globals()[sys.argv[1]]()


def capture_message():
    sentry_sdk.capture_message("test message", level="warning")


def division_by_zero():
    try:
        print(1 / 0)
    except Exception as err:
        sentry_sdk.capture_exception(err)


def custom_exception():
    try:
        custom_exc.raise_exc()
    except Exception as err:
        sentry_sdk.capture_exception(err)


if __name__ == "__main__":
    main()
