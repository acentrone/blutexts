"use client";

import { useEffect } from "react";

/**
 * Reveal-on-scroll for marketing pages.
 *
 * Watches every `.marketing-page .rise` and `.marketing-page .rise-group`
 * in the document. When one enters the viewport, adds `.in`, which the
 * stylesheet uses to animate it (and its children, for `.rise-group`)
 * from opacity:0 / translateY → opacity:1 / translateY(0).
 *
 * Identical behavior to the inline script in the original Claude Design
 * homepage source — extracted into a hook so every marketing page
 * (homepage, /pricing, /integrations, /solutions/*) gets it without
 * copy-pasting the observer.
 */
export function useRiseOnScroll() {
  useEffect(() => {
    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((e) => {
          if (e.isIntersecting) {
            e.target.classList.add("in");
            io.unobserve(e.target);
          }
        });
      },
      { threshold: 0.1, rootMargin: "0px 0px -60px 0px" }
    );
    document
      .querySelectorAll(".marketing-page .rise, .marketing-page .rise-group")
      .forEach((el) => io.observe(el));
    return () => io.disconnect();
  }, []);
}
