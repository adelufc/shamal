import type { ButtonHTMLAttributes } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost";
};

export function Button({ variant = "primary", className = "", ...props }: Props) {
  const look =
    variant === "primary"
      ? "bg-olive text-paper hover:opacity-90"
      : "border border-line bg-card text-ink hover:bg-olive-soft";
  return (
    <button
      className={`inline-flex items-center justify-center rounded-md px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50 ${look} ${className}`}
      {...props}
    />
  );
}
