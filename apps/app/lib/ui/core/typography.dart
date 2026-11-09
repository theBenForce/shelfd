import 'package:flutter/material.dart';

class AppTypography {
  AppTypography._();

  static const String editorialSerifFamily = 'Newsreader';
  static const String humanistSansFamily = 'Work Sans';
  static const String technicalMonoFamily = 'JetBrains Mono';

  static const List<String> serifFallbacks = [
    'Newsreader',
    'Georgia',
    'Charter',
    'Cambria',
    'Times New Roman',
    'serif',
  ];

  static const List<String> sansFallbacks = [
    'Work Sans',
    'Plus Jakarta Sans',
    'Inter',
    '-apple-system',
    'BlinkMacSystemFont',
    'Segoe UI',
    'Roboto',
    'Helvetica Neue',
    'Arial',
    'sans-serif',
  ];

  static const List<String> monoFallbacks = [
    'JetBrains Mono',
    'ui-monospace',
    'SFMono-Regular',
    'Menlo',
    'Monaco',
    'Consolas',
    'monospace',
  ];

  static TextStyle displaySerif({
    double fontSize = 32.0,
    FontWeight fontWeight = FontWeight.w600,
    Color? color,
    double height = 1.2,
    double letterSpacing = -0.5,
    FontStyle? fontStyle,
  }) {
    return TextStyle(
      fontFamily: editorialSerifFamily,
      fontFamilyFallback: serifFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: height,
      letterSpacing: letterSpacing,
      fontStyle: fontStyle,
    );
  }

  static TextStyle titleSerif({
    double fontSize = 22.0,
    FontWeight fontWeight = FontWeight.w600,
    Color? color,
    double? height,
    FontStyle? fontStyle,
    double letterSpacing = -0.2,
  }) {
    return TextStyle(
      fontFamily: editorialSerifFamily,
      fontFamilyFallback: serifFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: height,
      fontStyle: fontStyle,
      letterSpacing: letterSpacing,
    );
  }

  static TextStyle readerBodySerif({
    double fontSize = 18.0,
    FontWeight fontWeight = FontWeight.w400,
    Color? color,
    double lineHeight = 1.65,
    FontStyle? fontStyle,
  }) {
    return TextStyle(
      fontFamily: editorialSerifFamily,
      fontFamilyFallback: serifFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: lineHeight,
      fontStyle: fontStyle,
    );
  }

  static TextStyle bodySans({
    double fontSize = 15.0,
    FontWeight fontWeight = FontWeight.w400,
    Color? color,
    double lineHeight = 1.5,
    FontStyle? fontStyle,
    double? letterSpacing,
  }) {
    return TextStyle(
      fontFamily: humanistSansFamily,
      fontFamilyFallback: sansFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: lineHeight,
      fontStyle: fontStyle,
      letterSpacing: letterSpacing,
    );
  }

  static TextStyle labelCaps({
    double fontSize = 11.0,
    FontWeight fontWeight = FontWeight.w600,
    Color? color,
    double letterSpacing = 0.8,
    double? height,
  }) {
    return TextStyle(
      fontFamily: humanistSansFamily,
      fontFamilyFallback: sansFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      letterSpacing: letterSpacing,
      height: height,
    );
  }

  static TextStyle captionSans({
    double fontSize = 12.0,
    FontWeight fontWeight = FontWeight.w400,
    Color? color,
    double? height,
    FontStyle? fontStyle,
  }) {
    return TextStyle(
      fontFamily: humanistSansFamily,
      fontFamilyFallback: sansFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: height,
      fontStyle: fontStyle,
    );
  }

  static TextStyle mono({
    double fontSize = 12.0,
    FontWeight fontWeight = FontWeight.w500,
    Color? color,
    double height = 1.4,
    double? letterSpacing,
  }) {
    return TextStyle(
      fontFamily: technicalMonoFamily,
      fontFamilyFallback: monoFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: height,
      letterSpacing: letterSpacing,
      fontFeatures: const [FontFeature.tabularFigures()],
    );
  }

  static TextStyle readerText({
    required double fontSize,
    required double lineHeight,
    required bool isSerif,
    required Color color,
  }) {
    return TextStyle(
      fontFamily: isSerif ? editorialSerifFamily : humanistSansFamily,
      fontFamilyFallback: isSerif ? serifFallbacks : sansFallbacks,
      fontSize: fontSize,
      height: lineHeight,
      color: color,
    );
  }
}

