"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { AuthForm, AuthFooter, Field } from "@/components/auth/auth-form";
import { useAuthStore } from "@/stores/auth-store";

export default function RegisterPage() {
  const router = useRouter();
  const register = useAuthStore((state) => state.register);
  const pending = useAuthStore((state) => state.pending);
  const error = useAuthStore((state) => state.error);

  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [workspaceName, setWorkspaceName] = useState("");

  return (
    <AuthForm
      title="Buat workspace baru"
      subtitle="Pendaftar pertama menjadi OWNER workspace dan bisa mengundang anggota lain."
      submitLabel="Daftar"
      pending={pending}
      error={error}
      onSubmit={async () => {
        if (await register({ fullName, email, password, workspaceName })) router.replace("/catalog");
      }}
      footer={<AuthFooter mode="register" />}
    >
      <Field id="fullName" label="Nama lengkap" value={fullName} onChange={setFullName} autoComplete="name" />
      <Field id="workspaceName" label="Nama workspace" value={workspaceName} onChange={setWorkspaceName} />
      <Field id="email" label="Email" type="email" value={email} onChange={setEmail} autoComplete="email" />
      <Field
        id="password"
        label="Kata sandi"
        type="password"
        value={password}
        onChange={setPassword}
        autoComplete="new-password"
        placeholder="minimal 8 karakter"
      />
    </AuthForm>
  );
}
