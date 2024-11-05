import { useCallback, useEffect, useRef } from "react";

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Category } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Star } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { Badge } from "../../../components/ui/badge";

export default function ScriptAccordion({
  items,
  selectedScript,
  setSelectedScript,
}: {
  items: Category[];
  selectedScript: string | null;
  setSelectedScript: (script: string | null) => void;
}) {
  const [expandedItem, setExpandedItem] = useState<string | undefined>(
    undefined,
  );
  const linkRefs = useRef<{ [key: string]: HTMLAnchorElement | null }>({});
  
  const handleAccordionChange = (value: string | undefined) => {
    setExpandedItem(value);
  };

  const handleSelected = useCallback(
    (title: string) => {
      setSelectedScript(title);
    },
    [setSelectedScript],
  );

  useEffect(() => {
    if (selectedScript) {
      const category = items.find((category) =>
        category.expand.items.some((script) => script.title === selectedScript),
      );
      if (category) {
        setExpandedItem(category.catagoryName);
        handleSelected(selectedScript);
      }
    }
  }, [selectedScript, items, handleSelected]);
  return (
    <Accordion
      type="single"
      value={expandedItem}
      onValueChange={handleAccordionChange}
      collapsible
    >
      {items.map((category) => (
        <AccordionItem
          key={category.id + ":category"}
          value={category.catagoryName}
          className={cn("sm:text-md flex flex-col border-none", {
            "rounded-lg bg-accent/30": expandedItem === category.catagoryName,
          })}
        >
          <AccordionTrigger
            className={cn(
              "duration-250 rounded-lg transition ease-in-out hover:-translate-y-1 hover:scale-105 hover:bg-accent",
              { "": expandedItem === category.catagoryName },
            )}
          >
            <div className="mr-2 flex w-full items-center justify-between">
              <span className="pl-2">{category.catagoryName} </span>
              <span className="rounded-full bg-gray-200 px-2 py-1 text-xs text-muted-foreground hover:no-underline dark:bg-blue-800/20">
                {category.expand.items.length}
              </span>
            </div>{" "}
          </AccordionTrigger>
          <AccordionContent
            data-state={
              expandedItem === category.catagoryName ? "open" : "closed"
            }
            className="pt-0"
          >
            {category.expand.items
              .slice()
              .sort((a, b) => a.title.localeCompare(b.title))
              .map((script, index) => (
                <div key={index}>
                  <Link
                    href={{
                      pathname: "/scripts",
                      query: { id: script.title },
                    }}
                    prefetch={false}
                    className={`flex cursor-pointer items-center justify-between gap-1 px-1 py-1 text-muted-foreground hover:rounded-lg hover:bg-accent/60 hover:dark:bg-accent/20 ${
                      selectedScript === script.title
                        ? "rounded-lg bg-accent font-semibold dark:bg-accent/30 dark:text-white"
                        : ""
                    }`}
                    onClick={() => handleSelected(script.title)}
                    ref={(el) => {
                      linkRefs.current[script.title] = el;
                    }}
                  >
                    <Image
                      src={script.logo}
                      height={16}
                      width={16}
                      unoptimized
                      onError={(e) =>
                        ((e.currentTarget as HTMLImageElement).src =
                          "/logo.png")
                      }
                      alt={script.title}
                      className="mr-1 w-4 h-4 rounded-full"
                    />
                    <span className="flex items-center gap-2">
                      {script.title}
                      {script.isMostViewed && (
                        <Star className="h-3 w-3 text-yellow-500"></Star>
                      )}
                    </span>
                    <Badge
                      className={cn(
                        "ml-auto w-[37.69px] justify-center text-center",
                        {
                          "text-primary/75": script.item_type === "VM",
                          "text-yellow-500/75": script.item_type === "LXC",
                          "border-none": script.item_type === "",
                          hidden: !["VM", "LXC", ""].includes(script.item_type),
                        },
                      )}
                    >
                      {script.item_type}
                    </Badge>
                  </Link>
                </div>
              ))}
          </AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  );
}
