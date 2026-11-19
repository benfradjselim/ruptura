"""Tests for pin-bundle-digests.py without network: digests are faked.

  python3 scripts/test_pin_bundle_digests.py
"""
import importlib.util
import os
import unittest

spec = importlib.util.spec_from_file_location('pin', os.path.join(os.path.dirname(__file__), 'pin-bundle-digests.py'))
pin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pin)

D = 'sha256:' + 'a' * 64
CSV = """metadata:
  annotations:
    containerImage: ghcr.io/benfradjselim/ruptura-operator:v1.4.9
spec:
  install:
    spec:
      deployments:
        - spec:
            template:
              spec:
                containers:
                  - image: ghcr.io/benfradjselim/ruptura-operator:v1.4.9
                    env:
                      - name: RELATED_IMAGE_RUPTURA
                        value: docker.io/selimbf/ruptura:v8.7.9
                      - name: RELATED_IMAGE_RUPTURA_CONSOLE_PLUGIN
                        value: ghcr.io/benfradjselim/ruptura-console-plugin:plugin-v8.7.9
  relatedImages:
    - name: operator
      image: ghcr.io/benfradjselim/ruptura-operator:v1.4.9
    - name: ruptura
      image: docker.io/selimbf/ruptura:v8.7.9
    - name: console-plugin
      image: ghcr.io/benfradjselim/ruptura-console-plugin:plugin-v8.7.9
  icon: []
"""


class PinTest(unittest.TestCase):
    def setUp(self):
        self.calls = []
        pin.digest = lambda registry, repo, tag: self.calls.append((registry, repo, tag)) or D

    def test_pins_container_image_annotation_and_every_reference(self):
        text = ''.join(pin.pin(CSV.splitlines(True)))
        pin.check(text)
        self.assertIn('containerImage: ghcr.io/benfradjselim/ruptura-operator@' + D, text)
        self.assertNotIn(':v1.4.9', text)
        self.assertNotIn(':plugin-v8.7.9', text)
        self.assertEqual(len(set(self.calls)), 3)

    def test_refuses_a_tag_left_in_the_annotation(self):
        text = ''.join(pin.pin(CSV.splitlines(True)))
        text = text.replace('containerImage: ghcr.io/benfradjselim/ruptura-operator@' + D,
                            'containerImage: ghcr.io/benfradjselim/ruptura-operator:v1.4.9')
        with self.assertRaises(SystemExit):
            pin.check(text)


if __name__ == '__main__':
    unittest.main()
