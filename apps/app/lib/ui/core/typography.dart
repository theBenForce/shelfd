import 'package:flutter/material.dart';
import 'tokens.dart';

class AppTypography {
  AppTypography._();

  static const List<String> serifFallbacks = [
    'Charter',
    'Georgia',
    'Cambria',
    'Times New Roman',
    'serif',
  ];

  static const List<String> sansFallbacks = [
    '-apple-system',
    'BlinkMacSystemFont',
    'Segoe UI',
    'Roboto',
    'Helvetica Neue',
    'Arial',
    'sans-serif',
  ];

  static TextStyle titleSerif({
    double fontSize = 22.0,
    FontWeight fontWeight = FontWeight.w600,
    Color color = AppTokens.charcoalInk,
  }) {
    return TextStyle(
      fontFamily: 'Georgia',
      fontFamilyFallback: serifFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      letterSpacing: -0.2,
    );
  }

  static TextStyle bodySans({
    double fontSize = 15.0,
    FontWeight fontWeight = FontWeight.w400,
    Color color = AppTokens.mutedCopy,
    double lineHeight = 1.5,
  }) {
    return TextStyle(
      fontFamilyFallback: sansFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      height: lineHeight,
    );
  }

  static TextStyle labelCaps({
    double fontSize = 11.0,
    FontWeight fontWeight = FontWeight.w600,
    Color color = AppTokens.mutedCopy,
  }) {
    return TextStyle(
      fontFamilyFallback: sansFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      letterSpacing: 0.8,
    );
  }

  static TextStyle captionSans({
    double fontSize = 12.0,
    FontWeight fontWeight = FontWeight.w400,
    Color color = AppTokens.mutedCopy,
  }) {
    return TextStyle(
      fontFamilyFallback: sansFallbacks,
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
    );
  }

  static TextStyle readerText({
    required double fontSize,
    required double lineHeight,
    required bool isSerif,
    required Color color,
  }) {
    return TextStyle(
      fontFamily: isSerif ? 'Georgia' : null,
      fontFamilyFallback: isSerif ? serifFallbacks : sansFallbacks,
      fontSize: fontSize,
      height: lineHeight,
      color: color,
    );
  }
}

