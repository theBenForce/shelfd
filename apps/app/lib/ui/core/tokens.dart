import 'package:flutter/material.dart';

class AppTokens {
  AppTokens._();

  // Touch & Sizing Invariants
  static const double minTouchTarget = 48.0;
  static const double maxReadingWidth = 720.0;
  static const double maxSearchWidth = 860.0;
  static const double maxLibraryWidth = 1800.0;
  static const double sidebarWidth = 240.0;
  static const double mobileBreakpoint = 640.0;
  static const double tabletBreakpoint = 1024.0;

  // Spacing Scale (8px baseline)
  static const double space4 = 4.0;
  static const double space8 = 8.0;
  static const double space12 = 12.0;
  static const double space16 = 16.0;
  static const double space20 = 20.0;
  static const double space24 = 24.0;
  static const double space32 = 32.0;
  static const double space48 = 48.0;
  static const double space64 = 64.0;

  // Radii
  static const double radiusSm = 4.0;
  static const double radiusMd = 8.0;
  static const double radiusLg = 12.0;
  static const double radiusPill = 999.0;

  // Warm Editorial Bone Colors
  static const Color boneBackground = Color(0xFFFBFBFA);
  static const Color boneSurface = Color(0xFFF9F9F8);
  static const Color boneContainer = Color(0xFFF3F4F3);
  static const Color charcoalInk = Color(0xFF111111);
  static const Color mutedCopy = Color(0xFF444748);
  static const Color crispBorder = Color(0xFFEAEAEA);

  // Sepia Palette
  static const Color sepiaBackground = Color(0xFFF4ECD8);
  static const Color sepiaSurface = Color(0xFFEDE3CB);
  static const Color sepiaInk = Color(0xFF5B4636);
  static const Color sepiaCopy = Color(0xFF6E5644);
  static const Color sepiaBorder = Color(0xFFE0D4BA);

  // Dark / OLED Palette
  static const Color darkOledBackground = Color(0xFF121212);
  static const Color darkOledSurface = Color(0xFF1E1E1E);
  static const Color darkOledInk = Color(0xFFE0E0E0);
  static const Color darkOledCopy = Color(0xFFAAAAAA);
  static const Color darkOledBorder = Color(0xFF2A2A2A);

  // Status & Match Badges
  static const Color matchBadgeBg = Color(0xFFEDF3EC);
  static const Color matchBadgeText = Color(0xFF346538);

  // Stitch Audiobook Obsidian & Amber Palette
  static const Color obsidianBackground = Color(0xFF141315);
  static const Color obsidianContainer = Color(0xFF1C1B1D);
  static const Color obsidianSurface = Color(0xFF201F21);
  static const Color obsidianElevated = Color(0xFF2B292C);
  static const Color obsidianBorder = Color(0xFF2E2C30);
  static const Color amberAccent = Color(0xFFE5A967);
  static const Color amberGlow = Color(0xFFFFC589);
  static const Color terracottaAccent = Color(0xFFD97736);
  static const Color antiqueCream = Color(0xFFF3C99D);

  // Tactile Scriptorium — Editorial Accents (Burnt Sienna & Library Green)
  static const Color siennaPrimary = Color(0xFF8E4A26);
  static const Color siennaContainer = Color(0xFFAA603A);
  static const Color siennaLightTint = Color(0xFFF7ECE6);
  static const Color siennaDarkAccent = Color(0xFFD98A5F);
  static const Color libraryGreen = Color(0xFF3A5A40);
  static const Color libraryGreenContainer = Color(0xFFBCE0C1);
  static const Color libraryGreenDark = Color(0xFF81B28A);

  // Tactile Scriptorium — Warm Parchment & Archival Bone Hierarchy
  static const Color warmParchment = Color(0xFFFBF9F4);
  static const Color warmSurfaceLowest = Color(0xFFFFFFFF);
  static const Color warmSurfaceLow = Color(0xFFF5F3EE);
  static const Color warmSurfaceContainer = Color(0xFFEFECE6);
  static const Color warmSurfaceHigh = Color(0xFFE9E6DF);
  static const Color warmSurfaceHighest = Color(0xFFE2DFD8);
  static const Color espressoInk = Color(0xFF1E1B18);
  static const Color espressoSecondary = Color(0xFF53433C);
  static const Color espressoMuted = Color(0xFF756860);
  static const Color ghostBorder = Color(0xFFE3DDD3);
  static const Color subtleGhostBorder = Color(0xFFEDE8DF);

  // Tactile Scriptorium — Sepia Archival Vellum Extensions
  static const Color sepiaContainerLow = Color(0xFFEFE6D1);
  static const Color sepiaContainer = Color(0xFFE6DBC2);
  static const Color sepiaContainerHigh = Color(0xFFDCD0B4);
  static const Color sepiaContainerHighest = Color(0xFFD2C4A5);
  static const Color sepiaMuted = Color(0xFF866D59);
  static const Color sepiaSubtleBorder = Color(0xFFE8DEC6);

  // Tactile Scriptorium — Warm Dark Walnut Hierarchy
  static const Color walnutBackground = Color(0xFF161412);
  static const Color walnutSurface = Color(0xFF1E1B18);
  static const Color walnutContainerLow = Color(0xFF23201C);
  static const Color walnutContainer = Color(0xFF292521);
  static const Color walnutElevated = Color(0xFF322D28);
  static const Color walnutHighest = Color(0xFF3B352F);
  static const Color walnutInk = Color(0xFFEDE7DF);
  static const Color walnutSecondary = Color(0xFFBDB3A7);
  static const Color walnutMuted = Color(0xFF8C8277);
  static const Color walnutBorder = Color(0xFF332E29);
  static const Color walnutSubtleBorder = Color(0xFF282420);

  // Semantic Status & Badge Tokens
  static const Color progressBadgeBg = Color(0xFFF7ECE6);
  static const Color progressBadgeText = Color(0xFF8E4A26);
  static const Color completedBadgeBg = Color(0xFFEDF3EC);
  static const Color completedBadgeText = Color(0xFF3A5A40);
  static const Color errorBgLight = Color(0xFFFEF2F2);
  static const Color errorBorderLight = Color(0xFFFECACA);
  static const Color errorTextLight = Color(0xFFB91C1C);
  static const Color errorBgDark = Color(0xFF2D1618);
  static const Color errorBorderDark = Color(0xFF5C262A);
  static const Color errorTextDark = Color(0xFFFCA5A5);
}
