import type { InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from "react";

const field = "w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink outline-none focus:border-olive";

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={`${field} ${props.className ?? ""}`} {...props} />;
}

export function TextArea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={`${field} min-h-28 ${props.className ?? ""}`} {...props} />;
}

export function Label({ children }: { children: ReactNode }) {
  return <label className="grid gap-1 text-sm">{children}</label>;
}
