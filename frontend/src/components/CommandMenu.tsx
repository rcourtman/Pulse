import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Category } from "@/lib/types";
import { cn } from "@/lib/utils";
import Image from "next/image";
import { useRouter } from "next/navigation";
import React, { useEffect } from "react";
import { Button } from "./ui/button";
import { DialogTitle } from "./ui/dialog";

const sortCategories = (categories: Category[]): Category[] => {
  return categories.sort((a: Category, b: Category) => {
    if (
      a.catagoryName === "Proxmox VE Tools" &&
      b.catagoryName !== "Proxmox VE Tools"
    ) {
      return -1;
    } else if (
      a.catagoryName !== "Proxmox VE Tools" &&
      b.catagoryName === "Proxmox VE Tools"
    ) {
      return 1;
    } else {
      return a.catagoryName.localeCompare(b.catagoryName);
    }
  });
};

export default function CommandMenu() {
  const [open, setOpen] = React.useState(false);
  const [links, setLinks] = React.useState<Category[]>([]);
  const router = useRouter();
  const [isLoading, setIsLoading] = React.useState(false);

  React.useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((open) => !open);
      }
    };

    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, []);

  const fetchCategories = async () => {
    setIsLoading(true);
    fetch("api/categories")
      .then((response) => response.json())
      .then((categories) => {
        const sortedCategories = sortCategories(categories);
        setLinks(sortedCategories);
        setIsLoading(false);
      })
      .catch((error) => {
        setIsLoading(false);
        console.error(error)
      });
  };

  return (
    <>
      <Button
        variant="outline"
        className={cn(
          "relative h-9 w-full justify-start rounded-[0.5rem] bg-muted/50 text-sm font-normal text-muted-foreground shadow-none sm:pr-12 md:w-40 lg:w-64",
        )}
        onClick={() => {
          fetchCategories();
          setOpen(true)
        }}
      >
        <span className="inline-flex">Search scripts...</span>
        <kbd className="pointer-events-none absolute right-[0.3rem] top-[0.45rem] hidden h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium opacity-100 sm:flex">
          <span className="text-xs">⌘</span>K
        </kbd>
      </Button>
      <CommandDialog open={open} onOpenChange={setOpen}>
          <DialogTitle className="sr-only">Search scripts</DialogTitle>
        <CommandInput placeholder="search for a script..." />
        <CommandList>
          <CommandEmpty>{isLoading ? "Loading..." : "No scripts found."}</CommandEmpty>
          {links.map((category) => (
            <CommandGroup
              key={"category:" + category.catagoryName}
              heading={category.catagoryName}
            >
              {category.expand.items.map((script) => (
                <CommandItem
                  key={"script:" + script.id}
                  value={script.title}
                  onSelect={() => {
                    setOpen(false);
                    router.push(`/scripts?id=${script.title}`);
                  }}
                >
                  <div className="flex gap-2" onClick={() => setOpen(false)}>
                    <Image
                      src={script.logo}
                      unoptimized
                      height={16}
                      onError={(e) =>
                        ((e.currentTarget as HTMLImageElement).src =
                          "/logo.png")
                      }
                      width={16}
                      alt=""
                      className="h-5 w-5"
                    />
                    <span>{script.title}</span>
                    <span className="text-sm text-muted-foreground">
                      {script.item_type}
                    </span>
                  </div>
                </CommandItem>
              ))}
            </CommandGroup>
          ))}
        </CommandList>
      </CommandDialog>
    </>
  );
}
