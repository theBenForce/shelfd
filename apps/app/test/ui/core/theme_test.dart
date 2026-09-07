import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/core/tokens.dart';

void main() {
  group('Theme and Tokens Tests', () {
    test('Tokens define required accessible touch targets and colors', () {
      expect(AppTokens.minTouchTarget, 48.0);
      expect(AppTokens.maxReadingWidth, 720.0);
      expect(AppTokens.boneBackground, const Color(0xFFFBFBFA));
      expect(AppTokens.charcoalInk, const Color(0xFF111111));
      expect(AppTokens.sepiaBackground, const Color(0xFFF4ECD8));
      expect(AppTokens.darkOledBackground, const Color(0xFF121212));
    });

    test('ThemeData builders produce valid themes with proper scaffold colors', () {
      final bone = AppTheme.buildTheme(ReadingThemeMode.bone);
      expect(bone.scaffoldBackgroundColor, AppTokens.boneBackground);
      expect(bone.colorScheme.primary, AppTokens.charcoalInk);

      final sepia = AppTheme.buildTheme(ReadingThemeMode.sepia);
      expect(sepia.scaffoldBackgroundColor, AppTokens.sepiaBackground);
      expect(sepia.colorScheme.primary, AppTokens.sepiaInk);

      final dark = AppTheme.buildTheme(ReadingThemeMode.dark);
      expect(dark.scaffoldBackgroundColor, AppTokens.darkOledBackground);
      expect(dark.colorScheme.primary, AppTokens.darkOledInk);
    });
  });
}
