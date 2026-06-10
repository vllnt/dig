import { Faq } from "@/components/landing/faq";
import { Features } from "@/components/landing/features";
import { FinalCta } from "@/components/landing/final-cta";
import { Hero } from "@/components/landing/hero";
import { HowItWorks } from "@/components/landing/how-it-works";
import { Principles } from "@/components/landing/principles";
import { Problem } from "@/components/landing/problem";

export default function LandingPage() {
  return (
    <>
      <Hero />
      <Problem />
      <HowItWorks />
      <Features />
      <Principles />
      <Faq />
      <FinalCta />
    </>
  );
}
