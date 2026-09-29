const eastern = /[\u0660-\u0669\u06f0-\u06f9\uff10-\uff19]/g;

export function verificationDigits(value: string) {
  const letters = /[A-Za-z]/.test(value);
  const digits = value
    .replace(eastern, (digit) => {
      const code = digit.charCodeAt(0);
      if (code >= 0xff10) return String(code - 0xff10);
      if (code >= 0x06f0) return String(code - 0x06f0);
      return String(code - 0x0660);
    })
    .replace(/\D/g, "")
    .slice(0, 6);
  return { digits, letters };
}
