import binascii
import os
import struct

from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import padding
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes


class AesEncoder:
    def __init__(self, key: bytes):
        self.key = binascii.unhexlify(key)

    @staticmethod
    def generate_key(length: int = 32) -> bytes:
        if length not in {16, 24, 32}:
            raise ValueError("Invalid key length. Must be 16, 24, or 32 bytes.")
        return binascii.hexlify(os.urandom(length))

    @staticmethod
    def is_valid_key(key: bytes) -> bool:
        try:
            key_bytes = binascii.unhexlify(key)
            return len(key_bytes) in {16, 24, 32}
        except binascii.Error:
            return False

    def encrypt(self, data: bytes) -> str:
        iv = os.urandom(16)
        padder = padding.PKCS7(128).padder()
        data = padder.update(data) + padder.finalize()
        cipher = Cipher(
            algorithms.AES(self.key),
            modes.CBC(iv),
            backend=default_backend(),
        )
        encryptor = cipher.encryptor()
        ct = encryptor.update(data) + encryptor.finalize()
        return binascii.hexlify(struct.pack("<H", 16) + iv + ct).decode()

    def decrypt(self, data: bytes) -> bytes:
        data = binascii.unhexlify(data)
        iv_size = struct.unpack("<H", data[:2])[0]
        iv = data[2 : 2 + iv_size]
        ct = data[2 + iv_size :]
        cipher = Cipher(
            algorithms.AES(self.key),
            modes.CBC(iv),
            backend=default_backend(),
        )
        decryptor = cipher.decryptor()
        pt = decryptor.update(ct) + decryptor.finalize()
        unpadder = padding.PKCS7(128).unpadder()
        return unpadder.update(pt) + unpadder.finalize()
