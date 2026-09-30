import type { ButtonHTMLAttributes } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost" | "danger";
};

export function Button({ variant = "primary", className = "", ...props }: Props) {
  const look =
    variant === "primary"
      ? "bg-olive text-white hover:bg-[#25402F]"
      : variant === "danger"
        ? "border border-[#A52D35] bg-[#FFF0F1] text-[#A52D35]"
        : "border border-[#7B877D] bg-card text-ink hover:bg-olive-soft";
  return (
    <button
      className={`inline-flex min-h-11 items-center justify-center rounded-lg px-3 text-base disabled:cursor-not-allowed disabled:opacity-50 ${look} ${className}`}
      {...props}
    />
  );
}
