"use client";
import { cn } from "@/lib/utils";
import { CheckIcon, ClipboardIcon } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Card } from "./card";

export default function CodeCopyButton({
  children,
}: {
  children: React.ReactNode;
}) {
  const [hasCopied, setHasCopied] = useState(false);
  const isMobile = window.innerWidth <= 640;

  useEffect(() => {
    if (hasCopied) {
      setTimeout(() => {
        setHasCopied(false);
      }, 2000);
    }
  }, [hasCopied]);

  const handleCopy = (type: string, value: any) => {
    navigator.clipboard.writeText(value);

    setHasCopied(true);

    let warning = localStorage.getItem("warning");

    if (warning === null) {
      localStorage.setItem("warning", "1");
      setTimeout(() => {
        toast.error(
          "Be careful when copying scripts from the internet. Always remember check the source!",
          { duration: 8000 },
        );
      }, 500);
    }

    // toast.success(`copied ${type} to clipboard`, {
    //   icon: <ClipboardCheck className="h-4 w-4" />,
    // });
  };

  return (
    <div className="mt-4 flex">
      <Card className="flex items-center overflow-x-auto bg-primary-foreground pl-4">
        <div className="overflow-x-auto whitespace-pre-wrap text-nowrap break-all pr-4 text-sm">
          {!isMobile && children ? children : "Copy install command"}
        </div>
        <div
          className={cn(" right-0 cursor-pointer bg-muted px-3 py-4")}
          onClick={() => handleCopy("install command", children)}
        >
          {hasCopied ? (
            <CheckIcon className="h-4 w-4" />
          ) : (
            <ClipboardIcon className="h-4 w-4" />
          )}
          <span className="sr-only">Copy</span>
        </div>
      </Card>
    </div>
  );
}
