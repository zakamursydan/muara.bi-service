"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { AuthForm, AuthFooter, Field } from "@/components/auth/auth-form";
import { useAuthStore } from "@/stores/auth-store";

export default function LoginPage() {
  const router = useRouter();
  const login = useAuthStore((state) => state.login);
  const pending = useAuthStore((state) => state.pending);
  const error = useAuthStore((state) => state.error);

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  return (
    <AuthForm
      title="Masuk ke workspace"
      subtitle="Gunakan akun yang sudah diundang ke workspace muara.bi."
      submitLabel="Masuk"
      pending={pending}
      error={error}
      onSubmit={async () => {
        if (await login({ email, password })) router.replace("/catalog");
      }}
      footer={<AuthFooter mode="login" />}
    >
      <Field id="email" label="Email" type="email" value={email} onChange={setEmail} autoComplete="email" />
      <Field
        id="password"
        label="Kata sandi"
        type="password"
        value={password}
        onChange={setPassword}
        autoComplete="current-password"
      />
    </AuthForm>
  );
}
