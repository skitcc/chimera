import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('sanitize', Path(__file__).with_name('sanitize-auth-artifacts.py'))
sanitize = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sanitize)

class SanitizationTest(unittest.TestCase):
    def test_nested_http_credentials(self):
        value = {'request': {'current_password': 'random-password', 'code': '123456'},
                 'reply': {'token': 'jwt-value'}, 'status': 201}
        result = sanitize.scrub(value)
        self.assertEqual(result['request']['current_password'], '[REDACTED]')
        self.assertEqual(result['request']['code'], '[REDACTED]')
        self.assertEqual(result['reply']['token'], '[REDACTED]')
        self.assertEqual(result['status'], 201)

    def test_report_identifiers_and_durations_survive(self):
        self.assertEqual(sanitize.scrub('a123456b-attachment.txt'), 'a123456b-attachment.txt')
        self.assertEqual(sanitize.scrub('<testcase time="0.123456"/>'), '<testcase time="0.123456"/>')

    def test_text_jwt_otp_and_env_secrets(self):
        sanitize.SENSITIVE.append('generated-environment-secret')
        result = sanitize.scrub('Bearer eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIxIn0.signature code 654321 generated-environment-secret')
        self.assertNotIn('eyJ', result)
        self.assertNotIn('654321', result)
        self.assertNotIn('generated-environment-secret', result)

if __name__ == '__main__':
    unittest.main()
