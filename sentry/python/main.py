import os
import sys

import custom_exc
import sentry_sdk
import sentry_sdk.tracing_utils


def main():
    sentry_sdk.init(dsn=os.environ["DSN"], send_default_pii=True)
    if len(sys.argv) < 2:
        print("missing mandatory argument")
        sys.exit(1)
    sentry_sdk.get_current_scope()._propagation_context = sentry_sdk.tracing_utils.PropagationContext(
        trace_id="noop_trace_id",
        span_id="noop_span_id"
    )
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
