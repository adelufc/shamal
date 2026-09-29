"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { ApiError, api } from "@/components/admin-shell";
import { verificationDigits } from "@/lib/verification-code";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [challenge, setChallenge] = useState("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function submitPassword(event: FormEvent) {
    event.preventDefault();
    setError("");
    setPending(true);
    try {
      const result = await api<{ mfa_required: boolean; challenge_id?: string }>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password, context: "staff" }),
      });
      if (result.mfa_required && result.challenge_id) {
        setChallenge(result.challenge_id);
      } else {
        router.replace("/admin/neighborhoods");
      }
    } catch (caught) {
      setError(caught instanceof ApiError ? caught.message : "تعذر الدخول.");
    } finally {
      setPending(false);
    }
  }

  async function submitCode(event: FormEvent) {
    event.preventDefault();
    setError("");
    setPending(true);
    try {
      const { digits, letters } = verificationDigits(code);
      if (letters || digits.length !== 6) {
        setError("أدخل الرمز المكوّن من 6 أرقام لهذه اللحظة. القيمة الطويلة في الدليل ليست رمز الدخول.");
        return;
      }
      await api("/api/v1/auth/mfa", {
        method: "POST",
        body: JSON.stringify({ challenge_id: challenge, code: digits }),
      });
      router.replace("/admin/neighborhoods");
    } catch (caught) {
      setError(caught instanceof ApiError ? caught.message : "رمز التحقق غير مقبول.");
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="mx-auto grid min-h-screen max-w-md content-center gap-4 px-4">
      <a href="/" className="text-sm">
        شمال العقاري
      </a>
      <h1 className="text-2xl font-semibold">دخول فريق العمل</h1>
      <p className="text-sm text-ink/70">البريد ثم كلمة المرور، ثم رمز من 6 أرقام لهذه اللحظة. القيمة الطويلة في الدليل ليست هذا الرمز.</p>
      {challenge === "" ? (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={submitPassword}>
          <Label>
            البريد
            <Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} required autoComplete="username" />
          </Label>
          <Label>
            كلمة المرور
            <Input type="password" value={password} onChange={(event) => setPassword(event.target.value)} required autoComplete="current-password" />
          </Label>
          <Button disabled={pending}>{pending ? "جارٍ التحقق…" : "متابعة"}</Button>
        </form>
      ) : (
        <form className="grid gap-3 rounded-lg border border-line bg-card p-4" onSubmit={submitCode}>
          <Label>
            رمز من 6 أرقام
            <Input
              inputMode="numeric"
              autoComplete="off"
              pattern="[0-9]*"
              value={code}
              onChange={(event) => {
                const next = verificationDigits(event.target.value);
                if (next.letters) {
                  setCode("");
                  setError("أدخل الرمز المكوّن من 6 أرقام لهذه اللحظة. القيمة الطويلة في الدليل ليست رمز الدخول.");
                  return;
                }
                setError("");
                setCode(next.digits);
              }}
              required
            />
          </Label>
          <Button disabled={pending}>{pending ? "جارٍ التحقق…" : "دخول"}</Button>
        </form>
      )}
      {error ? <p className="text-sm text-warn">{error}</p> : null}
    </main>
  );
}
