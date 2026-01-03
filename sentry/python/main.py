import logging
import os
import sys

import custom_exc
import sentry_sdk
import sentry_sdk.tracing_utils
from sentry_sdk.integrations.logging import LoggingIntegration


logger = logging.getLogger(__name__)


def main():
    logging.basicConfig(level=logging.INFO)
    sentry_sdk.init(
        dsn=os.environ["DSN"],
        send_default_pii=True,
        release="test",
        integrations=[LoggingIntegration(level=logging.INFO)],
    )
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


def with_breadcrumbs():
    logger.info("info message with arg: %s", (42, "string argument"))
    logger.warning("warning message with extra", extra={"extra_key": 42})
    sentry_sdk.capture_message("test message", level="fatal")


def raise_new_during_except():
    try:
        try:
            print(1 / 0)
        except Exception as err:
            raise RuntimeError("other exception") from err
    except Exception as err:
        sentry_sdk.capture_exception(err)


def raise_same_during_except():
    try:
        try:
            print(1 / 0)
        except Exception as err:
            raise err
    except Exception as err:
        sentry_sdk.capture_exception(err)


def raise_same_during_capture():
    try:
        try:
            print(1 / 0)
        except Exception as err:
            sentry_sdk.capture_exception(err)
            raise err
    except Exception as err:
        sentry_sdk.capture_exception(err)


if __name__ == "__main__":
    main()
