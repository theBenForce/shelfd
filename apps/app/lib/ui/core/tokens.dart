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
}
