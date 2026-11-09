import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/core/tokens.dart';
import 'package:shelf/ui/core/typography.dart';

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
      expect(bone.colorScheme.onSurfaceVariant, AppTokens.mutedCopy);
      expect(bone.textTheme.bodyMedium?.color, AppTokens.charcoalInk);

      final sepia = AppTheme.buildTheme(ReadingThemeMode.sepia);
      expect(sepia.scaffoldBackgroundColor, AppTokens.sepiaBackground);
      expect(sepia.colorScheme.primary, AppTokens.sepiaInk);
      expect(sepia.colorScheme.onSurfaceVariant, AppTokens.sepiaCopy);
      expect(sepia.textTheme.bodyMedium?.color, AppTokens.sepiaInk);

      final dark = AppTheme.buildTheme(ReadingThemeMode.dark);
      expect(dark.scaffoldBackgroundColor, AppTokens.darkOledBackground);
      expect(dark.colorScheme.primary, AppTokens.darkOledInk);
      expect(dark.colorScheme.onSurfaceVariant, AppTokens.darkOledCopy);
      expect(dark.textTheme.bodyMedium?.color, AppTokens.darkOledInk);
    });

    test('Tactile Scriptorium tokens and ShelfdColors extension are registered per theme mode', () {
      expect(AppTokens.siennaPrimary, const Color(0xFF8E4A26));
      expect(AppTokens.libraryGreen, const Color(0xFF3A5A40));
      expect(AppTokens.warmParchment, const Color(0xFFFBF9F4));
      expect(AppTokens.walnutBackground, const Color(0xFF161412));

      final boneTheme = AppTheme.buildTheme(ReadingThemeMode.bone);
      final sepiaTheme = AppTheme.buildTheme(ReadingThemeMode.sepia);
      final darkTheme = AppTheme.buildTheme(ReadingThemeMode.dark);

      expect(boneTheme.extension<ShelfdColors>(), ShelfdColors.bone);
      expect(sepiaTheme.extension<ShelfdColors>(), ShelfdColors.sepia);
      expect(darkTheme.extension<ShelfdColors>(), ShelfdColors.dark);

      final copied = ShelfdColors.bone.copyWith(accentPrimary: AppTokens.libraryGreen);
      expect(copied.accentPrimary, AppTokens.libraryGreen);
      expect(copied.background, ShelfdColors.bone.background);

      final lerpedStart = ShelfdColors.bone.lerp(ShelfdColors.dark, 0.0);
      final lerpedEnd = ShelfdColors.bone.lerp(ShelfdColors.dark, 1.0);
      expect(lerpedStart.isDark, isFalse);
      expect(lerpedEnd.isDark, isTrue);
      expect(lerpedEnd.background, ShelfdColors.dark.background);
    });

    testWidgets('ShelfdColors.of(context) and context.shelfdColors resolve active palette and fallback', (tester) async {
      ShelfdColors? resolved;
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.dark),
          home: Builder(
            builder: (context) {
              resolved = context.shelfdColors;
              return const SizedBox.shrink();
            },
          ),
        ),
      );
      expect(resolved, isNotNull);
      expect(resolved!.isDark, isTrue);
      expect(resolved!.background, AppTokens.walnutBackground);
      expect(resolved!.ink, AppTokens.walnutInk);

      // Bare MaterialApp fallback without ShelfdColors extension
      await tester.pumpWidget(
        MaterialApp(
          theme: ThemeData.dark(),
          home: Builder(
            builder: (context) {
              resolved = AppTheme.colorsOf(context);
              return const SizedBox.shrink();
            },
          ),
        ),
      );
      expect(resolved!.isDark, isTrue);
      expect(resolved!.background, ShelfdColors.dark.background);
    });

    test('AppTypography configures Newsreader, Work Sans, and JetBrains Mono with tabular figures', () {
      final monoStyle = AppTypography.mono(fontSize: 13, fontWeight: FontWeight.w600);
      expect(monoStyle.fontFamily, 'JetBrains Mono');
      expect(monoStyle.fontSize, 13);
      expect(monoStyle.fontWeight, FontWeight.w600);
      expect(monoStyle.fontFeatures, contains(const FontFeature.tabularFigures()));

      final displayStyle = AppTypography.displaySerif(fontSize: 28);
      expect(displayStyle.fontFamily, 'Newsreader');
      expect(displayStyle.fontSize, 28);

      final readerSerif = AppTypography.readerBodySerif();
      expect(readerSerif.fontFamily, 'Newsreader');

      final bodyStyle = AppTypography.bodySans();
      expect(bodyStyle.fontFamily, 'Work Sans');
    });
  });
}
