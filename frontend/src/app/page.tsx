"use client";
import AnimatedGradientText from "@/components/ui/animated-gradient-text";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import Particles from "@/components/ui/particles";
import { Separator } from "@/components/ui/separator";
import { basePath } from "@/config/siteConfig";
import { cn } from "@/lib/utils";
import { ArrowRightIcon } from "lucide-react";
import { useTheme } from "next-themes";
import Link from "next/link";
import { useEffect, useState } from "react";
import { FaGithub, FaDiscord } from "react-icons/fa";

function CustomArrowRightIcon() {
  return <ArrowRightIcon className="h-4 w-4" />;
}

export default function Page() {
  const { theme } = useTheme();
  const [color, setColor] = useState("#000000");

  useEffect(() => {
    setColor(theme === "dark" ? "#ffffff" : "#000000");
  }, [theme]);

  return (
    <div className="relative w-full">
      <Particles
        className="absolute inset-0 -z-40"
        quantity={100}
        ease={80}
        color={color}
        refresh
      />

      {/* Header */}
      <header className="sticky top-0 z-50 bg-background/80 backdrop-blur-lg">
        <div className="container mx-auto flex flex-col items-center gap-4 px-4 py-4 sm:flex-row sm:justify-between">
          {/* Logo */}
          <Link href="/" className="flex items-center gap-2 font-bold text-lg">
            Proxmox VE Helper-Scripts
          </Link>

          {/* Suchleiste */}
          <input
            type="text"
            placeholder="Search scripts..."
            className="w-full sm:w-1/3 px-4 py-2 rounded-md bg-gray-800 text-gray-300 border border-gray-700"
          />

          {/* Icons */}
          <div className="flex items-center gap-4">
            <FaGithub className="w-5 h-5 text-gray-300" />
            <FaDiscord className="w-5 h-5 text-gray-300" />
          </div>
        </div>
      </header>

      {/* "Scripts by Tteck"-Button */}
      <div className="fixed top-16 right-4 sm:top-4 z-40">
        <Dialog>
          <DialogTrigger>
            <div className="flex items-center bg-purple-600 text-white px-4 py-2 rounded-lg">
              ❤️ Scripts by Tteck
            </div>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Thank You!</DialogTitle>
              <DialogDescription>
                A big thank you to Tteck and the many contributors who have
                made this project possible. Your hard work is truly appreciated
                by the entire Proxmox community!
              </DialogDescription>
            </DialogHeader>
          </DialogContent>
        </Dialog>
      </div>

      {/* Hauptinhalt */}
      <main className="container mx-auto mt-32 px-4">
        <h1 className="text-5xl sm:text-7xl font-bold text-center">
          Make managing your Homelab a breeze
        </h1>
        <p className="text-lg sm:text-xl text-center mt-6">
          We are a community-driven initiative that simplifies the setup of
          Proxmox Virtual Environment (VE). Originally created by{" "}
          <a href="https://github.com/tteck" target="_blank">
            tteck
          </a>
          , these scripts automate and streamline the process of creating and
          configuring Linux containers (LXC) and virtual machines (VMs) on
          Proxmox VE.
        </p>
        <div className="flex justify-center mt-8">
          <Link href="/scripts">
            <Button
              size="lg"
              variant="expandIcon"
              Icon={CustomArrowRightIcon}
              iconPlacement="right"
            >
              View Scripts
            </Button>
          </Link>
        </div>
      </main>
    </div>
  );
}
