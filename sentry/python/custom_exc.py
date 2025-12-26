class CustomException(Exception):
    pass


def raise_exc():
    raise CustomException("exception message")
