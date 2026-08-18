//+------------------------------------------------------------------+
//|                                    TradingViewNewsFilter.mqh     |
//|                    Copyright 2026, tradingview-calendar-go       |
//|                https://github.com/Nosvemos/tradingview-calendar-go|
//+------------------------------------------------------------------+
#property copyright "TradingView-Calendar-Go"
#property link      "https://github.com/Nosvemos/tradingview-calendar-go"
#property strict

//+------------------------------------------------------------------+
//| Struct representing an incoming economic event parsed from CSV   |
//+------------------------------------------------------------------+
struct TVNewsEvent
{
   string currency;
   int    minutesLeft;
   string impact;
   datetime releaseTime;
   string title;
   string forecast;
   string previous;
};

//+------------------------------------------------------------------+
//| TradingViewNewsFilter Class                                      |
//+------------------------------------------------------------------+
class CTradingViewNewsFilter
{
private:
   string m_fileName;
   datetime m_lastReadTime;

public:
   CTradingViewNewsFilter(string fileName = "ff_news_filter.csv")
   {
      m_fileName = fileName;
      m_lastReadTime = 0;
   }

   // Checks if a high-impact news event is upcoming within 'minutesBefore' or passed within 'minutesAfter'
   bool IsNewsRestricted(string symbolCurrency, int minutesBefore = 30, int minutesAfter = 15, string minImpact = "High")
   {
      int fileHandle = FileOpen(m_fileName, FILE_READ|FILE_CSV|FILE_ANSI, ',');
      if(fileHandle == INVALID_HANDLE)
      {
         Print("[TV-NewsFilter] Notice: Unable to open ", m_fileName, " in Terminal MQL Files directory.");
         return false;
      }

      // Skip header line
      FileReadString(fileHandle); // Currency
      FileReadString(fileHandle); // MinutesLeft
      FileReadString(fileHandle); // Impact
      FileReadString(fileHandle); // Timestamp
      FileReadString(fileHandle); // Title
      FileReadString(fileHandle); // Forecast
      FileReadString(fileHandle); // Previous

      while(!FileIsEnding(fileHandle))
      {
         string curr = FileReadString(fileHandle);
         string minStr = FileReadString(fileHandle);
         string imp = FileReadString(fileHandle);
         string tsStr = FileReadString(fileHandle);
         string title = FileReadString(fileHandle);
         string forecast = FileReadString(fileHandle);
         string prev = FileReadString(fileHandle);

         if(curr == "" || minStr == "") continue;

         int minutesLeft = (int)StringToInteger(minStr);
         datetime releaseTime = (datetime)StringToInteger(tsStr);

         // Check if symbol matches base or quote currency
         if(StringFind(symbolCurrency, curr) >= 0)
         {
            if(IsImpactAboveOrEqual(imp, minImpact))
            {
               // If event is upcoming within minutesBefore (e.g. within 30 min)
               if(minutesLeft >= 0 && minutesLeft <= minutesBefore)
               {
                  Print("[TV-NewsFilter] Trading Restricted: Upcoming news on ", curr, " (", title, ") in ", minutesLeft, " min.");
                  FileClose(fileHandle);
                  return true;
               }

               // If event passed recently within minutesAfter (e.g. 15 min after)
               if(minutesLeft < 0 && MathAbs(minutesLeft) <= minutesAfter)
               {
                  Print("[TV-NewsFilter] Trading Restricted: Post-news cooldown on ", curr, " (", title, ") passed ", MathAbs(minutesLeft), " min ago.");
                  FileClose(fileHandle);
                  return true;
               }
            }
         }
      }

      FileClose(fileHandle);
      return false;
   }

private:
   bool IsImpactAboveOrEqual(string actual, string minimum)
   {
      int actWeight = GetImpactWeight(actual);
      int minWeight = GetImpactWeight(minimum);
      return actWeight >= minWeight;
   }

   int GetImpactWeight(string impact)
   {
      if(StringFind(impact, "High") >= 0) return 3;
      if(StringFind(impact, "Medium") >= 0) return 2;
      if(StringFind(impact, "Low") >= 0) return 1;
      return 0;
   }
};
